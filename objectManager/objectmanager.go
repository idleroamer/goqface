package goqface

import (
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

var oncePerConn sync.Map

// ObjectManager mangages objects and their path in this service
// following the dbus specification of Object Manager from rev 0.17
type objectManager struct {
	objectMap                map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	objectServices           map[dbus.ObjectPath]string
	objectNodes              map[string]bool
	interfacesAddedObservers []interface {
		OnInterfacesAdded(serviceName string, objectPath dbus.ObjectPath)
	}
	interfacesRemovedObservers []interface {
		OnInterfacesRemoved(serviceName string, objectPath dbus.ObjectPath)
	}
	adapter *objectManagerAdapter
}

type objectManagerAdapter struct {
	objectManager          *objectManager
	remoteObjects          map[string]dbus.BusObject
	conn                   *dbus.Conn
	objectPath             dbus.ObjectPath
	interfaceName          string
	dbusServiceNamePattern string
}

var (
	instance map[*dbus.Conn]*objectManager
)

// ObjectManager returns a singleton instance of the ObjectManger for this service
func ObjectManager(conn *dbus.Conn) *objectManager {
	once, _ := oncePerConn.LoadOrStore(conn, &sync.Once{})
	once.(*sync.Once).Do(func() {
		if instance == nil {
			instance = make(map[*dbus.Conn]*objectManager)
		}
		instance[conn] = &objectManager{}
		instance[conn].init(conn)
	})
	return instance[conn]
}

func (o *objectManager) init(conn *dbus.Conn) {
	o.adapter = &objectManagerAdapter{objectManager: o}
	o.adapter.conn = conn
	o.objectMap = make(map[dbus.ObjectPath]map[string]map[string]dbus.Variant)
	o.objectNodes = make(map[string]bool)
	o.adapter.objectPath = "/"
	o.adapter.interfaceName = "org.freedesktop.DBus.ObjectManager"
	o.objectServices = make(map[dbus.ObjectPath]string)
	o.adapter.remoteObjects = make(map[string]dbus.BusObject)

	o.adapter.dbusServiceNamePattern = os.Getenv("DBUS_SERVICE_NAME_PATTERN")
	if o.adapter.dbusServiceNamePattern == "" {
		o.adapter.dbusServiceNamePattern = "qface.service"
	}
	postfix := strings.ReplaceAll(conn.Names()[0], ".", "")
	postfix = strings.ReplaceAll(postfix, ":", "")
	conn.RequestName(o.adapter.dbusServiceNamePattern+".X"+postfix, dbus.NameFlagDoNotQueue)
	var services []string
	err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&services)
	if err != nil {
		log.Fatal("Failed to get list of owned names:", err)
	}

	conn.ExportWithMap(o.adapter, map[string]string{"GetManagedObjects": "GetManagedObjects"}, o.adapter.objectPath, o.adapter.interfaceName)
	conn.ExportWithMap(o.adapter, map[string]string{"Introspect": "Introspect"}, o.adapter.objectPath, "org.freedesktop.DBus.Introspectable")
	go o.watchSignals()
	conn.BusObject().AddMatchSignal("org.freedesktop.DBus", "NameOwnerChanged")
	for _, s := range services {
		if matched, _ := regexp.MatchString(o.adapter.dbusServiceNamePattern, s); matched {
			if serviceOwner, err := o.getNameOwner(s); err == nil {
				o.watchService(serviceOwner)
			} else {
				log.Printf("Failed to GetNameOwner of the service %v", s)
			}
		}
	}
}

func (o *objectManager) watchService(serviceOwner string) {
	o.adapter.remoteObjects[serviceOwner] = o.adapter.conn.Object(serviceOwner, o.adapter.objectPath)
	if err := o.adapter.conn.AddMatchSignal(dbus.WithMatchInterface(o.adapter.interfaceName), dbus.WithMatchMember("InterfacesAdded"),
		dbus.WithMatchSender(serviceOwner)); err != nil {
		log.Printf("Failed to watch signal InterfacesAdded of on service %v with error %v", serviceOwner, err)
	}
	if err := o.adapter.conn.AddMatchSignal(dbus.WithMatchInterface(o.adapter.interfaceName), dbus.WithMatchMember("InterfacesRemoved"),
		dbus.WithMatchSender(serviceOwner)); err != nil {
		log.Printf("Failed to watch signal InterfacesRemoved of on service %v with error %v", serviceOwner, err)
	}
	ch := make(chan *dbus.Call, 2)
	o.adapter.remoteObjects[serviceOwner].Go(o.adapter.interfaceName+".GetManagedObjects", 0, ch)
	select {
	case call := <-ch:
		if call.Err == nil {
			if objectPaths, ok := call.Body[0].(map[dbus.ObjectPath]map[string]map[string]dbus.Variant); ok {
				for k := range objectPaths {
					o.objectServices[k] = serviceOwner
					for _, observer := range o.interfacesAddedObservers {
						go observer.OnInterfacesAdded(serviceOwner, k)
					}
				}
			} else {
				log.Printf("Managed object of GetManagedObjects of service %s are not in expected format", serviceOwner)
			}
		} else {
			log.Printf("Failed to GetManagedObjects of service %s due to \"%s\"", serviceOwner, call.Err)
		}
	}
	log.Printf("service %s watched for managed objects!", serviceOwner)
}

func (o *objectManager) getNameOwner(service string) (string, error) {
	var uniqueId string
	err := o.adapter.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, service).Store(&uniqueId)
	return uniqueId, err
}

func (o *objectManager) removeService(serviceOwner string) {
	for k, v := range o.objectServices {
		if v == serviceOwner {
			delete(o.objectServices, k)
			for _, observer := range o.interfacesRemovedObservers {
				go observer.OnInterfacesRemoved(v, k)
			}
		}
	}
	log.Printf("Service %v is disconnected!", serviceOwner)
}

func (o *objectManager) watchSignals() {
	ch := make(chan *dbus.Signal)
	o.adapter.conn.Signal(ch)
	for v := range ch {
		if v.Name == o.adapter.interfaceName+".InterfacesAdded" {
			var objectPath dbus.ObjectPath
			var interfacesAndProperties map[string]map[string]dbus.Variant
			err := dbus.Store(v.Body, &objectPath, &interfacesAndProperties)
			if err == nil {
				if _, ok := o.objectServices[objectPath]; !ok {
					o.objectServices[objectPath] = v.Sender
				} else {
					log.Printf("Objectpath %s already registered, ignore service %s", objectPath, v.Sender)
				}
				for _, observer := range o.interfacesAddedObservers {
					go observer.OnInterfacesAdded(v.Sender, objectPath)
				}
			} else if err != nil {
				log.Print(err)
			}
		} else if v.Name == o.adapter.interfaceName+".InterfacesRemoved" {
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
					go observer.OnInterfacesRemoved(v.Sender, objectPath)
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
				if matched, _ := regexp.MatchString(o.adapter.dbusServiceNamePattern, name); matched {
					if newOwner != "" {
						o.watchService(newOwner)
					} else {
						o.removeService(oldOwner)
					}
				}
			} else {
				log.Print(err)
			}
		}
	}
}

// GetManagedObjects get list of managed object in this service
func (o *objectManager) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, *dbus.Error) {
	return o.objectMap, nil
}

func (o *objectManagerAdapter) Introspect() (string, *dbus.Error) {
	methods := introspect.Methods(o)
	i := 0
	for _, method := range methods {
		if method.Name == "GetManagedObjects" {
			methods[i] = method
			i++
		}
	}
	methods = methods[:i]
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
	for nodes := range o.objectManager.objectNodes {
		n.Children = append(n.Children, introspect.Node{
			Name: nodes,
		})
	}
	introspectable := string(introspect.NewIntrospectable(n))
	return introspectable, nil
}

func (o *objectManagerAdapter) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, *dbus.Error) {
	return o.objectManager.GetManagedObjects()
}

func (o *objectManagerAdapter) InterfacesAdded(objectPath dbus.ObjectPath, interfacesAndproperties map[string]map[string]dbus.Variant) {
	o.conn.Emit(o.objectPath, o.interfaceName+".InterfacesAdded", objectPath, interfacesAndproperties)
}

func (o *objectManagerAdapter) InterfacesRemoved(objectPath dbus.ObjectPath, interfaces []string) {
	o.conn.Emit(o.objectPath, o.interfaceName+".InterfacesRemoved", objectPath, interfaces)
}

// RegisterObject make an object at given object path known to other services
func (o *objectManager) RegisterObject(objectPath dbus.ObjectPath, interfacesAndproperties map[string]map[string]dbus.Variant) {
	if _, ok := o.objectMap[objectPath]; !ok {
		o.objectMap[objectPath] = interfacesAndproperties
		o.adapter.InterfacesAdded(objectPath, interfacesAndproperties)
		paths := strings.Split(string(objectPath), "/")
		if len(paths) > 2 {
			o.objectNodes[paths[1]] = true
		} else {
			log.Fatalf("Incorrect object path %s", objectPath)
		}
	} else {
		log.Fatalf("Can't register already registered object %s", objectPath)
	}

}

// UnregisterObject call to inform other clients a registred object is destructed
func (o *objectManager) UnregisterObject(objectPath dbus.ObjectPath, interfaces []string) {
	if _, ok := o.objectMap[objectPath]; ok {
		delete(o.objectMap, objectPath)
		o.adapter.InterfacesRemoved(objectPath, interfaces)
	} else {
		log.Fatalf("Can't unregister a not registered object %s", objectPath)
	}
}

func (o *objectManager) AddInterfacesAddedObserver(observer interface{ OnInterfacesAdded(string, dbus.ObjectPath) }) {
	found := false
	for i := range o.interfacesAddedObservers {
		if o.interfacesAddedObservers[i] == observer {
			found = true
			break
		}
	}
	if !found {
		o.interfacesAddedObservers = append(o.interfacesAddedObservers, observer)
	}
}

func (o *objectManager) RemoveInterfacesAddedObserver(observer interface{ OnInterfacesAdded(string, dbus.ObjectPath) }) bool {
	found := false
	for i := range o.interfacesAddedObservers {
		if o.interfacesAddedObservers[i] == observer {
			o.interfacesAddedObservers = append(o.interfacesAddedObservers[:i], o.interfacesAddedObservers[i+1:]...)
			found = true
		}
	}
	return found
}

func (o *objectManager) AddInterfacesRemovedObserver(observer interface{ OnInterfacesRemoved(string, dbus.ObjectPath) }) {
	found := false
	for i := range o.interfacesRemovedObservers {
		if o.interfacesRemovedObservers[i] == observer {
			found = true
			break
		}
	}
	if !found {
		o.interfacesRemovedObservers = append(o.interfacesRemovedObservers, observer)
	}
}

func (o *objectManager) RemoveInterfacesRemovedObserver(observer interface{ OnInterfacesRemoved(string, dbus.ObjectPath) }) bool {
	found := false
	for i := range o.interfacesRemovedObservers {
		if o.interfacesRemovedObservers[i] == observer {
			o.interfacesRemovedObservers = append(o.interfacesRemovedObservers[:i], o.interfacesRemovedObservers[i+1:]...)
			found = true
		}
	}
	return found
}

func (o *objectManager) ObjectService(objectPath dbus.ObjectPath) string {
	if val, ok := o.objectServices[objectPath]; ok {
		return val
	}
	return ""
}

func (o *objectManagerAdapter) signalsIntrospection() []introspect.Signal {
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
