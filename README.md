# goqface

A minimal [dbus](https://dbus.freedesktop.org/doc/dbus-tutorial.html#whatis) framework which auto-generates bindings for [godbus](https://github.com/godbus/dbus) based on the interface-definition-language [qface](https://doc.qt.io/QtIVI/idl-syntax.html).

## Architecture

`qface` describes APIs based on known concepts such as modules, interfaces, properties, structs, signals and enums. Based on these definitions `goqface` generates boiler-plate glue bindings for [godbus](https://github.com/godbus/dbus), leaving developers with only business logic implementation.

There are four main components in `goqface` architecture. 
* `Interface` declares the methods, properties and signals as described in qface
* `Base` implements the properties/signals of the `Interface` and is to be embedded in `Implementation` 
* `DBusAdapter` exports methods, properties and signals to bus from `service process`
* `DBusProxy` represents the `DBusAdapter` on the `client process`
* `Implementation` implements the `Interface` methods

where only the last components needs to be implemented and reset are auto-generated. 

![Class hierarchy](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.github.com/idleroamer/goqface/master/assets/class-hierarchy.puml)

## Initialization

The initialization sequence starts by `DBusAdapter` `export`ing an object to bus from `service process`. Then on the `client process`, given the bus name of service is known (achieved automatically by [Object Management](#Object-Management)), `DBusProxy` attempts to fetch all properties upon `ConnectToRemoteObject` call. Afterward the status of the connection to the service can be checked by the conventional [ready property](#ready-property)]. 
On a successful connection the `DBusProxy` is able to call `DBusAdapter` methods and listen to its signals and in turn inform the registered [`observers`](#observers).

![Initial Sequence](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.github.com/idleroamer/goqface/master/assets/initial-adapter-proxy-sequence.puml)

## Properties

Properties are available as defined in qface interface both in `DBusAdapter` and `DBusProxy`.
`DBusProxy` fetches all `properties` (given a successful connection) on `ConnectToRemoteObjec` method call. Properties are always in sync between `DBusProxy` and `DBusAdapter` by the mean of `PropertiesChanged` signal.

Given a property is not defined `readonly` in qface, its value might be changed by `DBusProxy`. See [Properties Checks](#Properties-Checks) on how to optionally verify the assigned value on `DBusAdapter`. 

![Property get set](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.github.com/idleroamer/goqface/master/assets/property-get-set-sequence.puml)

### Ready Property

`ready` is a conventional auxiliary property to be checked to ensure that the connection to remote-object was successful and the remote-object `DBusAdapter` is actually ready to handle method calls.

## Methods

Remote method calls are initiated by `DBusProxy` and invoke the corresponding `DBusAdapter` function. Beside normal code path [exceptions](#Exceptions) can be handled as well.

## Signals

Signals defined in qface interface may be invoked from `DBusAdapter` by calling the corresponding function. In turn signals are received by the `DBusProxy` side and registered [Observers](#Observers) are informed.

## Observers

`Observers` watch signals on `DBusProxy` as well as property changes on both `DBusAdapter` and `DBusProxy`. i.e `Observers` are informed in goroutines if watched events emitted.

![observers](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.github.com/idleroamer/goqface/master/assets/observers.puml)

### Exceptions

`methods` could handle unexpected inputs and states by returning an optional `dbus.Error`.


### Properties Checks

It is possible to block unexpected values assigned to `properties` and optionally return an `error` with more info to the client. Simply assign a struct to `DBusAdapter.Set<Property>Callback()` which implements `Set<Property>` interface.

## Go Generate

A python script is the code-generator for goqface. It is possible to integrate the code-generation in your go files by leveraging go tools.

Use import plus $GOPATH to locate the `codegen.py` and pass the required arguments to the call.
Optionally use `gofmt` to format the generated files.

```
import (
	goqface "github.com/idleroamer/goqface/objectManager"
)

//go:generate python3 $GOPATH/pkg/mod/github.com/idleroamer/goqface@v<VERSION>/codegen.py  --input <LIST_OF_INPUTS> --dependency [LIST_OF_DEPENDENCIES]
//go:generate gofmt -w <PATH_OF_GENERATED_FILES>
```
`--input` list of all qface input files to generate bindings for.

`--dependency` optional path to [interdependencies](#Module-Interdependency)

`--output` optional output path of generated files otherwise module name will be used as path.

### Dependencies

The goqface python dependency are defined in `requirement.txt` file. These dependencies needs to be installed once but nevertheless you can integrate this step as well into go generate.

```
import (
	goqface "github.com/idleroamer/goqface/objectManager"
)

//go:generate pip3 install -r $GOPATH/pkg/mod/github.com/idleroamer/goqface@v<VERSION>/requirements.txt
```

## Module Interdependency

Modules may import other modules via `import` keyword followed by the imported module name and version.
The goqface creates the necessary import annotation for each of input files so that it can be used in the client code. The annotations are stored in the `.go.annotate` yaml file next to each `.qface`.

**_NOTE:_** `go module` should be initialized (`go mod int`) where outputs are located for a proper import annotation.

## Object Management

Object management in goqface follows the dbus specification of [org.freedesktop.DBus.ObjectManager](https://dbus.freedesktop.org/doc/dbus-specification.html#standard-interfaces-objectmanager).
The root object `/` implements the `ObjectManager` interface which can be used to query list of objects in this service.

Besides `Object Manager` monitors all related objects on bus in order to figure out to which service a `DBusProxy` needs to connect to. See also [related services](#Related-Services).
As long as prerequisite are in place this is a seamless operation.

Prerequisite
* `DBusAdapter` and `DBusProxy` have the same interface name and object path (see [dbus-definition](https://dbus.freedesktop.org/doc/dbus-faq.html#idm39))
* Bus name of `DBusAdapter` is in expected format (see also [related services](#Related-Services))
* Service of `DBusAdpater` has a valid object manager under the root object `/` 

### Related Services
A predefined name pattern of bus name makes detection of related services possible. So that all related services and their objects life-cycle can be monitored. 

**_NOTE:_**  The "qface.registry." pattern is used in case "DBUS_SERVICE_NAME_PATTERN" environment variable not defined

### Life time of objects

`Close` will end the `DBusAdapter` service on bus.
Consequently `ready` property of `DBusProxy` will be set to false, given one rely on [Object Management](#Object-Management) instead of setting service name explicitly.

```
DBusAdapter.Close()
```

## Limitation

There are some limitation with regards to qface:
* keyword [Model](https://doc.qt.io/qt-5/model-view-programming.html) is not supported
* keyword [Flag](https://doc.qt.io/QtIVI/idl-syntax.html#enum-or-flag) is not supported
* extending feature is not supported