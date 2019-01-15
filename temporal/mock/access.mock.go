// Code generated by counterfeiter. DO NOT EDIT.
package mock

import (
	"sync"

	"github.com/RTradeLtd/ipfs-orchestrator/temporal"
)

type FakeAccessChecker struct {
	CheckIfUserHasAccessToNetworkStub        func(string, string) (bool, error)
	checkIfUserHasAccessToNetworkMutex       sync.RWMutex
	checkIfUserHasAccessToNetworkArgsForCall []struct {
		arg1 string
		arg2 string
	}
	checkIfUserHasAccessToNetworkReturns struct {
		result1 bool
		result2 error
	}
	checkIfUserHasAccessToNetworkReturnsOnCall map[int]struct {
		result1 bool
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeAccessChecker) CheckIfUserHasAccessToNetwork(arg1 string, arg2 string) (bool, error) {
	fake.checkIfUserHasAccessToNetworkMutex.Lock()
	ret, specificReturn := fake.checkIfUserHasAccessToNetworkReturnsOnCall[len(fake.checkIfUserHasAccessToNetworkArgsForCall)]
	fake.checkIfUserHasAccessToNetworkArgsForCall = append(fake.checkIfUserHasAccessToNetworkArgsForCall, struct {
		arg1 string
		arg2 string
	}{arg1, arg2})
	fake.recordInvocation("CheckIfUserHasAccessToNetwork", []interface{}{arg1, arg2})
	fake.checkIfUserHasAccessToNetworkMutex.Unlock()
	if fake.CheckIfUserHasAccessToNetworkStub != nil {
		return fake.CheckIfUserHasAccessToNetworkStub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.checkIfUserHasAccessToNetworkReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeAccessChecker) CheckIfUserHasAccessToNetworkCallCount() int {
	fake.checkIfUserHasAccessToNetworkMutex.RLock()
	defer fake.checkIfUserHasAccessToNetworkMutex.RUnlock()
	return len(fake.checkIfUserHasAccessToNetworkArgsForCall)
}

func (fake *FakeAccessChecker) CheckIfUserHasAccessToNetworkCalls(stub func(string, string) (bool, error)) {
	fake.checkIfUserHasAccessToNetworkMutex.Lock()
	defer fake.checkIfUserHasAccessToNetworkMutex.Unlock()
	fake.CheckIfUserHasAccessToNetworkStub = stub
}

func (fake *FakeAccessChecker) CheckIfUserHasAccessToNetworkArgsForCall(i int) (string, string) {
	fake.checkIfUserHasAccessToNetworkMutex.RLock()
	defer fake.checkIfUserHasAccessToNetworkMutex.RUnlock()
	argsForCall := fake.checkIfUserHasAccessToNetworkArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeAccessChecker) CheckIfUserHasAccessToNetworkReturns(result1 bool, result2 error) {
	fake.checkIfUserHasAccessToNetworkMutex.Lock()
	defer fake.checkIfUserHasAccessToNetworkMutex.Unlock()
	fake.CheckIfUserHasAccessToNetworkStub = nil
	fake.checkIfUserHasAccessToNetworkReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakeAccessChecker) CheckIfUserHasAccessToNetworkReturnsOnCall(i int, result1 bool, result2 error) {
	fake.checkIfUserHasAccessToNetworkMutex.Lock()
	defer fake.checkIfUserHasAccessToNetworkMutex.Unlock()
	fake.CheckIfUserHasAccessToNetworkStub = nil
	if fake.checkIfUserHasAccessToNetworkReturnsOnCall == nil {
		fake.checkIfUserHasAccessToNetworkReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 error
		})
	}
	fake.checkIfUserHasAccessToNetworkReturnsOnCall[i] = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakeAccessChecker) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.checkIfUserHasAccessToNetworkMutex.RLock()
	defer fake.checkIfUserHasAccessToNetworkMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeAccessChecker) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ temporal.AccessChecker = new(FakeAccessChecker)
