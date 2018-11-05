package ipfs

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RTradeLtd/ipfs-orchestrator/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

// NodeClient provides an interface to the base Docker client for controlling
// IPFS nodes
type NodeClient interface {
	// Nodes retrieves a list of active IPFS ndoes
	Nodes(ctx context.Context) (nodes []*NodeInfo, err error)

	// CreateNode activates a new IPFS node
	CreateNode(ctx context.Context, n *NodeInfo, opts NodeOpts) (err error)

	// StopNode shuts down an existing IPFS node
	StopNode(ctx context.Context, n *NodeInfo) (err error)

	// Watch initalizes a goroutine that tracks IPFS node events
	Watch(ctx context.Context) (<-chan Event, <-chan error)
}

type client struct {
	l *zap.SugaredLogger
	d *docker.Client

	ipfsImage string
	dataDir   string
}

// NewClient creates a new Docker Client from ENV values and negotiates the
// correct API version to use
func NewClient(logger *zap.SugaredLogger, ipfsOpts config.IPFS) (NodeClient, error) {
	d, err := docker.NewEnvClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to dockerd: %s", err.Error())
	}
	d.NegotiateAPIVersion(context.Background())

	// pull required images
	ipfsImage := "ipfs/go-ipfs:" + ipfsOpts.Version
	if _, err = d.ImagePull(context.Background(), ipfsImage, types.ImagePullOptions{}); err != nil {
		return nil, fmt.Errorf("failed to download IPFS image: %s", err.Error())
	}

	c := &client{
		l:         logger.Named("ipfs"),
		d:         d,
		ipfsImage: ipfsImage,
		dataDir:   ipfsOpts.DataDirectory,
	}

	// initialize directories
	os.MkdirAll(c.getDataDir(""), 0755)

	return c, nil
}

func (c *client) Nodes(ctx context.Context) ([]*NodeInfo, error) {
	ctrs, err := c.d.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	nodes := make([]*NodeInfo, 0)
	for _, container := range ctrs {
		n, err := newNode(container.ID, container.Names[0], container.Labels)
		if err != nil {
			continue
		}
		nodes = append(nodes, &n)
	}

	return nodes, nil
}

// NodeOpts declares options for starting up nodes
type NodeOpts struct {
	SwarmKey       []byte
	BootstrapPeers []string
	AutoRemove     bool
}

func (c *client) CreateNode(ctx context.Context, n *NodeInfo, opts NodeOpts) error {
	if n == nil || n.NetworkID == "" || opts.SwarmKey == nil {
		return errors.New("invalid configuration provided")
	}

	// set up directories
	os.MkdirAll(c.getDataDir(n.NetworkID), 0700)

	// write swarm.key to mount point
	if err := ioutil.WriteFile(
		c.getDataDir(n.NetworkID)+"/swarm.key",
		opts.SwarmKey, 0700,
	); err != nil {
		return fmt.Errorf("failed to write key: %s", err.Error())
	}

	// check peers
	bootstrap := opts.BootstrapPeers != nil && len(opts.BootstrapPeers) > 0
	peerBytes, _ := json.Marshal(opts.BootstrapPeers)

	var (
		containerName = "ipfs-" + n.NetworkID
		ports         = nat.PortMap{
			// public ports
			"4001": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: n.Ports.Swarm}},

			// private ports
			"5001": []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: n.Ports.API}},
			"8080": []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: n.Ports.Gateway}},
		}
		volumes = []string{
			c.getDataDir(n.NetworkID) + ":/data/ipfs",
		}
		labels = map[string]string{
			"network_id":      n.NetworkID,
			"data_dir":        c.getDataDir(n.NetworkID),
			"swarm_port":      n.Ports.Swarm,
			"api_port":        n.Ports.API,
			"gateway_port":    n.Ports.Gateway,
			"bootstrap_peers": string(peerBytes),
			"job_id":          n.JobID,
		}
		restartPolicy container.RestartPolicy
	)

	// set restart policy
	if !opts.AutoRemove {
		restartPolicy = container.RestartPolicy{
			Name: "unless-stopped",
		}
	}

	// create ipfs node container
	resp, err := c.d.ContainerCreate(
		ctx,
		&container.Config{
			Image: c.ipfsImage,
			Cmd: []string{
				"daemon", "--migrate=true", "--enable-pubsub-experiment",
			},
			Env: []string{
				"LIBP2P_FORCE_PNET=1", // enforce private networks
			},
			Labels:       labels,
			Tty:          true,
			AttachStdout: true,
			AttachStderr: true,
		},
		&container.HostConfig{
			AutoRemove:    opts.AutoRemove,
			RestartPolicy: restartPolicy,
			Binds:         volumes,
			PortBindings:  ports,

			// TODO: limit resources
			Resources: container.Resources{},
		},
		nil, containerName,
	)
	if err != nil {
		return fmt.Errorf("failed to instantiate node: %s", err.Error())
	}

	// check for warnings
	if len(resp.Warnings) > 0 {
		return fmt.Errorf("errors encountered: %s", strings.Join(resp.Warnings, "\n"))
	}

	// spin up node
	if err := c.d.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start ipfs node: %s", err.Error())
	}

	// wait for node to start
	if err := c.waitForNode(ctx, resp.ID); err != nil {
		return err
	}

	// bootstrap peers if required
	if bootstrap {
		if err := c.bootstrapNode(ctx, resp.ID, opts.BootstrapPeers...); err != nil {
			return err
		}
	}

	// assign node metadata
	n.DockerID = resp.ID
	n.ContainerName = containerName
	n.DataDir = c.getDataDir(n.NetworkID)
	return nil
}

func (c *client) StopNode(ctx context.Context, n *NodeInfo) error {
	if n == nil {
		return errors.New("invalid node")
	}

	// stop container
	timeout := time.Duration(10 * time.Second)
	if err := c.d.ContainerStop(ctx, n.DockerID, &timeout); err != nil {
		return err
	}

	// remove container
	c.d.ContainerRemove(ctx, n.DockerID, types.ContainerRemoveOptions{
		RemoveLinks:   true,
		RemoveVolumes: true,
	})

	// remove ipfs data
	return os.RemoveAll(c.getDataDir(n.NetworkID))
}

func (c *client) bootstrapNode(ctx context.Context, dockerID string, peers ...string) error {
	if peers == nil || len(peers) == 0 {
		return errors.New("no peers provided")
	}

	// remove default peers
	rmBootstrap := []string{"ipfs", "bootstrap", "rm", "--all"}
	exec, err := c.d.ContainerExecCreate(ctx, dockerID, types.ExecConfig{Cmd: rmBootstrap})
	if err != nil {
		return err
	}
	if err := c.d.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{}); err != nil {
		return err
	}

	// bootstrap custom peers
	bootstrap := []string{"ipfs", "bootstrap", "add"}
	exec, err = c.d.ContainerExecCreate(ctx, dockerID, types.ExecConfig{
		Cmd: append(bootstrap, peers...),
	})
	if err != nil {
		return fmt.Errorf("failed to init bootstrapping process with %s: %s", dockerID, err.Error())
	}

	return c.d.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
}

func (c *client) waitForNode(ctx context.Context, dockerID string) error {
	logs, err := c.d.ContainerLogs(ctx, dockerID, types.ContainerLogsOptions{
		ShowStdout: true,
		Follow:     true,
	})
	if err != nil {
		return err
	}
	defer logs.Close()

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("cancelled wait for %s", dockerID)
		default:
			if strings.Contains(scanner.Text(), "Daemon is ready") {
				return nil
			}
		}
	}

	return scanner.Err()
}

// Event is a node-related container event
type Event struct {
	Time   int64    `json:"time"`
	Status string   `json:"status"`
	Node   NodeInfo `json:"node"`
}

// Watch listens for specific container events
func (c *client) Watch(ctx context.Context) (<-chan Event, <-chan error) {
	var (
		events = make(chan Event)
		errs   = make(chan error)
	)

	go func() {
		defer close(errs)
		eventsCh, eventsErrCh := c.d.Events(ctx,
			types.EventsOptions{Filters: filters.NewArgs(
				filters.KeyValuePair{Key: "event", Value: "die"},
				filters.KeyValuePair{Key: "event", Value: "start"},
			)})

		for {
			select {
			case <-ctx.Done():
				break

			// pipe errors back
			case err := <-eventsErrCh:
				if err != nil {
					errs <- err
				}

			// report events
			case status := <-eventsCh:
				id := status.ID[:11]
				name := status.Actor.Attributes["name"]
				node, err := newNode(id, name, status.Actor.Attributes)
				if err != nil {
					continue
				}
				e := Event{Time: status.Time, Status: status.Status, Node: node}
				c.l.Infow("event received",
					"event", e)
				events <- e
			}
		}
	}()

	return events, errs
}

func (c *client) getDataDir(network string) string {
	p, _ := filepath.Abs(filepath.Join(c.dataDir, fmt.Sprintf("/data/ipfs/%s", network)))
	return p
}
