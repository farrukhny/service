// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codegen

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/ServiceWeaver/weaver/internal/config"
	"github.com/ServiceWeaver/weaver/runtime"
	"github.com/ServiceWeaver/weaver/runtime/protos"
	"go.opentelemetry.io/otel/trace"
)

// globalRegistry is the global registry used by Register and Registered.
var globalRegistry registry

// Register registers a Service Weaver component.
func Register(reg Registration) {
	if err := globalRegistry.register(reg); err != nil {
		panic(err)
	}
}

// Registered returns the components registered with Register.
func Registered() []*Registration {
	return globalRegistry.allComponents()
}

// Find returns the registration of the named component.
func Find(name string) (*Registration, bool) {
	return globalRegistry.find(name)
}

// registry is a repository for registered Service Weaver components.
// Entries are typically added to the default registry by calls
// to Register in init functions in code generated by "weaver generate".
type registry struct {
	m          sync.Mutex
	components map[reflect.Type]*Registration // the set of registered components, by their interface types
	byName     map[string]*Registration       // map from full component name to registration
}

// Registration is the configuration needed to register a Service Weaver component.
type Registration struct {
	Name      string       // full package-prefixed component name
	Iface     reflect.Type // interface type for the component
	Impl      reflect.Type // implementation type (struct)
	Routed    bool         // True if calls to this component should be routed
	Listeners []string     // the names of any weaver.Listeners
	NoRetry   []int        // indices of methods that should not be retried

	// Functions that return different types of stubs.
	LocalStubFn   func(impl any, caller string, tracer trace.Tracer) any
	ClientStubFn  func(stub Stub, caller string) any
	ServerStubFn  func(impl any, load func(key uint64, load float64)) Server
	ReflectStubFn func(func(method string, ctx context.Context, args []any, returns []any) error) any

	// RefData holds a string containing the result of MakeEdgeString(Name, Dst)
	// for all components named Dst used by this component.
	RefData string
}

// register registers a Service Weaver component. If the registry's close method was
// previously called, Register will fail and return a non-nil error.
func (r *registry) register(reg Registration) error {
	if err := verifyRegistration(reg); err != nil {
		return fmt.Errorf("Register(%q): %w", reg.Name, err)
	}

	r.m.Lock()
	defer r.m.Unlock()
	if old, ok := r.components[reg.Iface]; ok {
		return fmt.Errorf("component %s already registered for type %v when registering %v",
			reg.Name, old.Impl, reg.Impl)
	}
	if r.components == nil {
		r.components = map[reflect.Type]*Registration{}
	}
	if r.byName == nil {
		r.byName = map[string]*Registration{}
	}
	ptr := &reg
	r.components[reg.Iface] = ptr
	r.byName[reg.Name] = ptr
	return nil
}

func verifyRegistration(reg Registration) error {
	if reg.Iface == nil {
		return errors.New("missing component type")
	}
	if reg.Iface.Kind() != reflect.Interface {
		return errors.New("component type is not an interface")
	}
	if reg.Impl == nil {
		return errors.New("missing implementation type")
	}
	if reg.Impl.Kind() != reflect.Struct {
		return errors.New("implementation type is not a struct")
	}
	if reg.LocalStubFn == nil {
		return errors.New("nil LocalStubFn")
	}
	if reg.ClientStubFn == nil {
		return errors.New("nil ClientStubFn")
	}
	if reg.ServerStubFn == nil {
		return errors.New("nil ServerStubFn")
	}
	return nil
}

// allComponents returns all of the registered components, keyed by name.
func (r *registry) allComponents() []*Registration {
	r.m.Lock()
	defer r.m.Unlock()

	components := make([]*Registration, 0, len(r.components))
	for _, info := range r.components {
		components = append(components, info)
	}
	return components
}

func (r *registry) find(path string) (*Registration, bool) {
	r.m.Lock()
	defer r.m.Unlock()
	reg, ok := r.byName[path]
	return reg, ok
}

// ComponentConfigValidator checks that cfg is a valid configuration
// for the component type whose fully qualified name is given by path.
//
// TODO(mwhittaker): Move out of codegen package? It's not used by the
// generated code.
func ComponentConfigValidator(path, cfg string) error {
	info, ok := globalRegistry.find(path)
	if !ok {
		// Not for a known component.
		return nil
	}
	componentConfig := config.Config(reflect.New(info.Impl))
	if componentConfig == nil {
		return fmt.Errorf("unexpected configuration for component %v "+
			"that does not support configuration (add a "+
			"weaver.WithConfig[configType] embedded field to %v)",
			info.Name, info.Iface)
	}
	config := &protos.AppConfig{Sections: map[string]string{path: cfg}}
	if err := runtime.ParseConfigSection(path, "", config.Sections, componentConfig); err != nil {
		return fmt.Errorf("%v: bad config: %w", info.Iface, err)
	}
	return nil
}

// CallEdge records that fact that the Caller component uses the
// Callee component. Both types are types of the corresponding
// component interfaces.
type CallEdge struct {
	Caller reflect.Type
	Callee reflect.Type
}

// CallGraph returns the component call graph (as a list of CallEdge values).
func CallGraph() []CallEdge {
	var result []CallEdge
	for _, reg := range Registered() {
		impl := reg.Impl
		for i, n := 0, impl.NumField(); i < n; i++ {
			// Handle field with type weaver.Ref[T].
			ref := impl.Field(i).Type
			if ref.PkgPath() == "github.com/ServiceWeaver/weaver" &&
				strings.HasPrefix(ref.Name(), "Ref[") &&
				ref.Kind() == reflect.Struct &&
				ref.NumField() == 1 &&
				ref.Field(0).Name == "value" {
				result = append(result, CallEdge{reg.Iface, ref.Field(0).Type})
			}
		}
	}
	return result
}