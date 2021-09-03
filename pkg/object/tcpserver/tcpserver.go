package tcpserver

import (
	"fmt"
	"github.com/megaease/easegress/pkg/graceupdate"
	"github.com/megaease/easegress/pkg/protocol"
	"time"

	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/layer4stat"
)

const (
	// Category is the category of TCPServer.
	Category = supervisor.CategoryTrafficGate

	// Kind is the kind of HTTPServer.
	Kind = "TCPServer"

	checkFailedTimeout = 10 * time.Second

	topNum = 10

	stateNil     stateType = "nil"
	stateFailed  stateType = "failed"
	stateRunning stateType = "running"
	stateClosed  stateType = "closed"
)

func init() {
	supervisor.Register(&TCPServer{})
}

var (
	errNil = fmt.Errorf("")
	gnet   = graceupdate.Global
)

type (
	stateType string

	eventCheckFailed struct{}
	eventServeFailed struct {
		startNum uint64
		err      error
	}
	eventReload struct {
		nextSuperSpec *supervisor.Spec
	}
	eventClose struct{ done chan struct{} }

	TCPServer struct {
		runtime *runtime
	}

	// Status contains all status generated by runtime, for displaying to users.
	Status struct {
		Health string `yaml:"health"`

		State stateType `yaml:"state"`
		Error string    `yaml:"error,omitempty"`

		*layer4stat.Status
	}
)

// Category get object category: supervisor.CategoryTrafficGate
func (T *TCPServer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind get object kind: http server
func (T *TCPServer) Kind() string {
	return Kind
}

func (T *TCPServer) DefaultSpec() interface{} {
	return &Spec{
		KeepAlive:      true,
		MaxConnections: 10240,
	}
}

func (T *TCPServer) Status() *supervisor.Status {
	panic("implement me")
}

// Close http server
func (T *TCPServer) Close() {
	T.runtime.Close()
}

// Init initializes HTTPServer.
func (T *TCPServer) Init(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	panic("implement me")
}

// Inherit inherits previous generation of HTTPServer.
func (T *TCPServer) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper protocol.MuxMapper) {
	panic("implement me")
}
