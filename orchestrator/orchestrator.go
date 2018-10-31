package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	tcfg "github.com/RTradeLtd/config"
	"github.com/RTradeLtd/database"
	"github.com/RTradeLtd/database/models"

	"github.com/RTradeLtd/ipfs-orchestrator/config"
	"github.com/RTradeLtd/ipfs-orchestrator/ipfs"
	"github.com/RTradeLtd/ipfs-orchestrator/log"
	"github.com/RTradeLtd/ipfs-orchestrator/registry"
	"go.uber.org/zap"
)

// Orchestrator contains most primary application logic and manages node
// availability
type Orchestrator struct {
	l  *zap.SugaredLogger
	nm *models.IPFSNetworkManager

	client ipfs.NodeClient
	reg    *registry.NodeRegistry
	host   string
}

// New instantiates and bootstraps a new Orchestrator
func New(logger *zap.SugaredLogger, host string, c ipfs.NodeClient,
	ports config.Ports, pg tcfg.Database, dev bool) (*Orchestrator, error) {
	// bootstrap registry
	nodes, err := c.Nodes(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch nodes: %s", err.Error())
	}
	reg := registry.New(logger, ports, nodes...)

	// set up database connection
	dbm, err := database.Initialize(&tcfg.TemporalConfig{
		Database: pg,
	}, database.DatabaseOptions{
		SSLModeDisable: dev,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %s", err.Error())
	}

	return &Orchestrator{
		l:      logger,
		nm:     models.NewHostedIPFSNetworkManager(dbm.DB),
		client: c,
		reg:    reg,
	}, nil
}

// Run initalizes the orchestrator's background tasks
func (o *Orchestrator) Run(ctx context.Context) error {
	go o.client.Watch(ctx)
	go func() {
		select {
		case <-ctx.Done():
			if err := o.nm.DB.Close(); err != nil {
				o.l.Warnw("error occured closing database connection",
					"error", err)
			}
		}
	}()
	return nil
}

// NetworkUp intializes a node for given network
func (o *Orchestrator) NetworkUp(ctx context.Context, network string) error {
	if network == "" {
		return errors.New("invalid network name provided")
	}

	start := time.Now()
	jobID := generateID()
	l := log.NewProcessLogger(o.l, "network_up",
		"job_id", jobID,
		"network", network)
	l.Info("network up process started")

	// check if request is valid
	n, err := o.nm.GetNetworkByName(network)
	if err != nil {
		return fmt.Errorf("no network with name '%s' found", network)
	}
	l.Info("network retrieved from database",
		"network.db_id", n.ID)

	// set options based on database entry
	opts, err := getOptionsFromDatabaseEntry(n)
	if err != nil {
		return fmt.Errorf("failed to configure network: %s", err.Error())
	}

	// register node for network
	newNode := &ipfs.NodeInfo{NetworkID: network, JobID: jobID}
	if err := o.reg.Register(newNode); err != nil {
		return fmt.Errorf("failed to allocate resources for network '%s': %s", network, err)
	}

	// instantiate node
	l.Info("network registered, creating node")
	if err := o.client.CreateNode(ctx, newNode, opts); err != nil {
		return fmt.Errorf("failed to instantiate node for network '%s': %s", network, err)
	}
	l.Infow("node created",
		"node", newNode)

	// update network in database
	n.APIURL = o.host + ":" + newNode.Ports.API
	n.SwarmKey = string(opts.SwarmKey)
	n.Activated = time.Now()
	if err := o.nm.UpdateNetwork(n); err != nil {
		return fmt.Errorf("failed to update network '%s': %s", network, err)
	}

	l.Infow("network up process completed",
		"network_up.duration", time.Since(start))

	return nil
}

// NetworkDown brings a network offline
func (o *Orchestrator) NetworkDown(ctx context.Context, network string) error {
	if network == "" {
		return errors.New("invalid network name provided")
	}

	start := time.Now()
	jobID := generateID()
	l := log.NewProcessLogger(o.l, "network_down",
		"job_id", jobID,
		"network", network)
	l.Info("network up process started")

	// retrieve node from registry
	node, err := o.reg.Get(network)
	if err != nil {
		l.Info("could not find node in registry")
		return fmt.Errorf("failed to get node for network %s from registry: %s", network, err.Error())
	}

	// shut down node
	l.Info("network found, stopping node")
	if err := o.client.StopNode(ctx, &node); err != nil {
		l.Errorw("error occured while stopping node",
			"error", err,
			"node", node)
	}
	l.Infow("node stopped",
		"node", node)

	// deregister node
	if err := o.reg.Deregister(network); err != nil {
		l.Warnw("error occured while deregistering node",
			"node.network_id", node.NetworkID,
			"error", err)
	}

	// update network in database to indicate it is no longer active
	var t time.Time
	if err := o.nm.UpdateNetworkByName(network, map[string]interface{}{
		"activated": t,
		"api_url":   "",
	}); err != nil {
		l.Errorw("failed to update database entry for network",
			"err", err,
			"node.network_id", node.NetworkID)
		return fmt.Errorf("failed to update network '%s': %s", network, err)
	}

	l.Infow("network down process completed",
		"network_down.duration", time.Since(start))

	return nil
}
