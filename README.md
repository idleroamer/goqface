# goqface

A minimal [dbus](https://dbus.freedesktop.org/doc/dbus-tutorial.html#whatis) framework which generates interfaces for [godbus](https://github.com/godbus/dbus) based on interface-definition-language [qface](https://doc.qt.io/QtIVI/idl-syntax.html).

## Architecture

Based on qface interface definitions `goqface` generates glue bindings for [godbus](https://github.com/godbus/dbus), effectively leaving only concrete implementation of `methods` open.

There are four main components in `goqface` architecture. 
* `Interface` declares `methods` defined in qface files 
* `Implemenation` implements the `Interface` methods
* `DBusAdapter` exports `methods`, `properties` and `signals` to dbus from `service process`
* `DBusProxy` represents the `DBusAdapter` on the `client process`

`Properties` and `signals` are part of `DBusAdapter`, therefore one needs to integrate the `DBusAdapter` into `Implementation` to be able to access `properties` or emit `signals`. 

**_NOTE:_** Throughout examples and tests `DBusAdapter` is included as an anonymous nested struct in `Implementation`.

![Class hierarchy](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.github.com/idleroamer/goqface/master/assets/class-hierarchy.puml)

## Initialization

The initialization sequence starts by `DBusAdapter` `export`ing object to dbus. Then `DBusProxy` will attempt to fetch all `properties` upon `ConnectToRemoteObject` call, given the bus name of the `service` is known (should be achieved automatically by [Object Management](#Object-Management)).
Afterward the status of the connection to the service can be checked by the conventional [ready property](#ready-property)]. On successful connection the `DBusProxy` is able to call `DBusAdapter` methods besides it will watch the signals and inform the registered [`observers`](#observers).

![Initial Sequence](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.github.com/idleroamer/goqface/master/assets/initial-adapter-proxy-sequence.puml)

### Ready Property

`ready` is a conventional auxiliary property to be used by `DBusProxy` to ensure that the connection to remote-object was successful and the remote-object `DBusAdapter` is actually ready to handle method calls.

## Properties

Properties are available as defined in qface interface both in `DBusAdapter` and `DBusProxy`.
`DBusProxy` fetches all `properties` (given a successful connection) on `ConnectToRemoteObjec` method call. Properties are always in sync between `DBusProxy` and `DBusAdapter` by the mean of `PropertiesChanged` signal.

Given a property is not defined `readonly` in qface, its value might be changed by `DBusProxy`. See [Properties Checks](#Properties-Checks) on how to optionally verify the assigned value on `DBusAdapter`. 

![Property get set](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.github.com/idleroamer/goqface/master/assets/property-get-set-sequence.puml)

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

//go:generate python3 $GOPATH/pkg/mod/github.com/idleroamer/goqface@v<VERSION>/codegen.py --src <SOURCE_PATH_CONTAINING_QFACE_FILES> --input <LIST_OF_INPUTS>
//go:generate gofmt -w <PATH_OF_GENERATED_FILES>
```
`--src` argument is important for goqface to locate the [module interdependencies](#Module-Interdependency) otherwise current directory (where go generator file located) is taken. 

`--input` list of all qface input files to generate bindings for.

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
In order for `goqface` generator to be able to find the imported modules, 
the `@gomod` annotation on `imported module` needs to point to the path where generated files are located.

```
@gomod: "github.com/idleroamer/goqface/<PATH_TO_OUTPUT>/Foo/Yoo"
module Foo.Yoo 
```

`<PATH_TO_OUTPUT>` defined in [generation step](#go-generate).

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

One may end the a service lifetime on dbus by unregistering the `DBusAdapter`.

```
server, err := dbus.SessionBus()
goqface.ObjectManager(server).UnregisterObject(DBusAdapter.ObjectPath(), nil)
```