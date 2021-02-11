package main

//go:generate python3 ../../codegen.py --input AddressBook.qface
//go:generate gofmt -w Examples

import (
	"fmt"

	addressbook "github.com/idleroamer/goqface/_examples/AddressBook/Examples/AddressBook"

	"github.com/godbus/dbus/v5"
)

type AddressBookSignalWatcher struct {
}

func (c *AddressBookSignalWatcher) OnContactCreated(contact addressbook.Contact) {

}
func (c *AddressBookSignalWatcher) OnContactUpdateFailed(failureReason addressbook.FailureReason) {

}
func (c *AddressBookSignalWatcher) OnContactDeleted(contact addressbook.Contact) {

}
func (c *AddressBookSignalWatcher) OnContactUpdatedTo(index int, contact addressbook.Contact) {

}
func (c *AddressBookSignalWatcher) IsLoadedChanged(isLoaded bool) {

}
func (c *AddressBookSignalWatcher) CurrentContactChanged(currentContact addressbook.Contact) {

}
func (c *AddressBookSignalWatcher) ContactsChanged(contacts []addressbook.Contact) {

}
func (c *AddressBookSignalWatcher) IntValuesChanged(intValues []int) {

}
func (c *AddressBookSignalWatcher) MapOfContactChanged(mapOfC map[string]addressbook.Contact) {

}
func (c *AddressBookSignalWatcher) NestedChanged(nested addressbook.Nested) {

}
func (c *AddressBookSignalWatcher) ReadyChanged(ready bool) {

}

func main() {
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	signalWatcher := &AddressBookSignalWatcher{}
	proxy := &addressbook.AddressBookProxy{Conn: conn}
	proxy.Init(signalWatcher)
	proxy.ConnectToServer("goqface.addressbook")
	proxy.Setcontacts([]addressbook.Contact{addressbook.Contact{1, "JohnDoe", "TelNummer", 2}, addressbook.Contact{2, "MAxMusterman", "Handy", 234}})

	c := make(chan *dbus.Signal, 10)
	for v := range c {
		fmt.Println(v)
	}
}
