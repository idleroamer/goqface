package goqface

import (
	"errors"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

var once sync.Once

// ObjectManager mangages objects and their path in this service
// following the dbus specification of Object Manager from rev 0.17
type objectManager struct {
	objectMap                  map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	objectServices             map[dbus.ObjectPath]string
	interfacesAddedObservers   []interface{ OnInterfaceAdded(objectPath string) }
	interfacesRemovedObservers []interface{ OnInterfaceRemoved(objectPath string) }
	adapter                    *objectManagerAdapter
	conn                       *dbus.Conn
	objectPath                 dbus.ObjectPath
	interfaceName              string
	dbusServiceNamePattern     string
}

type objectManagerAdapter struct {
	objectManager *objectManager
}

var (
	instance map[*dbus.Conn]*objectManager
)

// ObjectManager returns a singleton instance of the ObjectManger for this service
func (o *objectManager) ObjectManager(conn *dbus.Conn) *objectManager {
	once.Do(func() {
		instance[conn].init(conn)
	})
	return instance[conn]
}

func (o *objectManager) init(conn *dbus.Conn) {
	o.conn = conn
	o.adapter = &objectManagerAdapter{objectManager: o}
	o.objectMap = make(map[dbus.ObjectPath]map[string]map[string]dbus.Variant)
	o.objectPath = "/"
	o.interfaceName = "org.freedesktop.DBus.ObjectManager"
	methods := introspect.Methods(o.adapter)

	o.dbusServiceNamePattern = os.Getenv("DBUS_SERVICE_NAME_PATTERN")
	conn.RequestName(o.dbusServiceNamePattern+".X"+conn.Names()[0], dbus.NameFlagDoNotQueue)
	var services []string
	err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&services)
	if err != nil {
		log.Fatal("Failed to get list of owned names:", err)
	}

	go o.watchSignals()
	conn.BusObject().AddMatchSignal("org.freedesktop.DBus", "NameOwnerChanged")
	for _, s := range services {
		o.watchService(s)
	}

	conn.Export(o, o.objectPath, o.interfaceName)
	n := &introspect.Node{
		Name: string(o.objectPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:    o.interfaceName,
				Methods: methods,
				Signals: o.signalsIntrospection(),
			},
		},
	}
	conn.Export(introspect.NewIntrospectable(n), o.objectPath,
		"org.freedesktop.DBus.Introspectable")
}

func (o *objectManager) watchService(service string) {
	currentService := false
	for _, names := range o.conn.Names() {
		currentService = currentService || service == names
	}
	if matched, _ := regexp.MatchString(o.dbusServiceNamePattern, service); !currentService && matched {
		var objectPaths map[dbus.ObjectPath]map[string]map[string]dbus.Variant
		remoteObj := o.conn.Object(service, "/")
		remoteObj.Call(o.interfaceName+".GetManagedObjects", 0).Store(&objectPaths)
		for k := range objectPaths {
			o.objectServices[k] = service
		}
		remoteObj.AddMatchSignal(o.interfaceName, "InterfacesAdded")
		remoteObj.AddMatchSignal(o.interfaceName, "InterfacesRemoved")
		log.Printf("serive %s is watched for managed objects", service)
	}
}

func (o *objectManager) watchSignals() {
	ch := make(chan *dbus.Signal)
	o.conn.Signal(ch)
	for v := range ch {
		if v.Name == o.interfaceName+"InterfacesAdded" {
			var objectPath dbus.ObjectPath
			var interfacesAndProperties map[string]map[string]dbus.Variant
			err := dbus.Store(v.Body, &objectPath, &interfacesAndProperties)
			if err == nil {
				if _, ok := o.objectServices[objectPath]; !ok {
					o.objectServices[objectPath] = v.Sender
				} else {
					log.Printf("Object path %s already registered, ignore service %s", objectPath, v.Sender)
				}
				for _, observer := range o.interfacesAddedObservers {
					go observer.OnInterfaceAdded(string(objectPath))
				}
			} else if err != nil {
				log.Print(err)
			}
		} else if v.Name == o.interfaceName+"InterfacesRemoved" {
			var objectPath dbus.ObjectPath
			var interfaces []string
			err := dbus.Store(v.Body, &objectPath, &interfaces)
			if err == nil {
				if value, ok := o.objectServices[objectPath]; ok {
					if value == v.Sender {
						delete(o.objectServices, objectPath)
					} else {
						log.Printf("Object path %s registered by service %s can't be removed by service %s", objectPath, value, v.Sender)
					}
				} else {
					log.Printf("Object path %s not registered, ignore removal signal from service %s", objectPath, v.Sender)
				}

				for _, observer := range o.interfacesRemovedObservers {
					go observer.OnInterfaceRemoved(string(objectPath))
				}
			} else if err != nil {
				log.Print(err)
			}
		} else if v.Name == "org.freedesktop.DBus.NameOwnerChanged" {
			var name string
			var oldOwner string
			var newOwner string
			err := dbus.Store(v.Body, &name, &oldOwner, &newOwner)
			if err == nil {
				o.watchService(name)
			} else if err != nil {
				log.Print(err)
			}
		}
	}
}

// GetManagedObjects get list of managed object in this service
func (o *objectManager) GetManagedObjects() map[dbus.ObjectPath]map[string]map[string]dbus.Variant {
	return o.objectMap
}

// RegisterObject make an object at given object path known to other services
func (o *objectManager) RegisterObject(objectPath dbus.ObjectPath, interfacesAndproperties map[string]map[string]dbus.Variant) {
	o.objectMap[objectPath] = interfacesAndproperties
	o.objectServices[objectPath] = o.conn.Names()[0]
	o.conn.Emit(o.objectPath, o.interfaceName+".InterfacesAdded", objectPath, interfacesAndproperties)
}

// UnregisterObject call to inform other clients a registred object is destructed
func (o *objectManager) UnregisterObject(objectPath dbus.ObjectPath, interfaces []string) {
	delete(o.objectMap, objectPath)
	delete(o.objectServices, objectPath)
	o.conn.Emit(o.objectPath, o.interfaceName+".InterfacesRemoved", objectPath, interfaces)
}

func (o *objectManager) AddInterfaceAddedObserver(observer interface{ OnInterfaceAdded(string) }) {
	o.interfacesAddedObservers = append(o.interfacesAddedObservers, observer)
}

func (o *objectManager) AddInterfaceRemovedObserver(observer interface{ OnInterfaceRemoved(string) }) bool {
	found := false
	for i := range o.interfacesRemovedObservers {
		if o.interfacesRemovedObservers[i] == observer {
			o.interfacesRemovedObservers = append(o.interfacesRemovedObservers[:i], o.interfacesRemovedObservers[i+1:]...)
			found = true
		}
	}
	return found
}

func (o *objectManager) ObjectService(objectPath dbus.ObjectPath) (string, error) {
	if val, ok := o.objectServices[objectPath]; ok {
		return val, nil
	}
	return "", errors.New("objectPath can't be found in registered services")
}

func (o *objectManager) signalsIntrospection() []introspect.Signal {
	t := reflect.TypeOf(o)
	signals := map[string][]string{"InterfacesAdded": {"objectPath", "interfacesAndProperties"},
		"InterfacesRemoved": {"objectPath", "interfaces"},
	}
	ms := make([]introspect.Signal, 0, len(signals))
	for k, v := range signals {
		signal, b := t.MethodByName(strings.Title(k))
		if !b {
			panic("something wrong in generated code")
		}
		var m introspect.Signal
		m.Name = k
		m.Args = make([]introspect.Arg, 0, signal.Type.NumIn())
		for j, param := range v {
			arg := introspect.Arg{Name: param, Type: dbus.SignatureOfType(signal.Type.In(j + 1)).String(), Direction: "out"}
			m.Args = append(m.Args, arg)
		}
		m.Annotations = make([]introspect.Annotation, 0)
		ms = append(ms, m)
	}
	return ms
}

func (o *objectManagerAdapter) GetManagedObjects() map[dbus.ObjectPath]map[string]map[string]dbus.Variant {
	return o.objectManager.GetManagedObjects()
}
