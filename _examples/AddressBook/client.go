package main

//go:generate python3 ../../generator/codegen.py --input AddressBook.qface
//go:generate gofmt -w Examples

import (
	"fmt"

	addressbook "github.com/idleroamer/goqface/_examples/AddressBook/Examples/AddressBook"

	"github.com/godbus/dbus/v5"
)

type AddressBookProxySignals struct {
}

func (c *AddressBookProxySignals) OnContactCreated(contact addressbook.Contact) {

}
func (c *AddressBookProxySignals) OnContactUpdateFailed(failureReason addressbook.FailureReason) {

}
func (c *AddressBookProxySignals) OnContactDeleted(contact addressbook.Contact) {

}
func (c *AddressBookProxySignals) OnContactUpdatedTo(index int, contact addressbook.Contact) {

}
func (c *AddressBookProxySignals) IsLoadedChanged(isLoaded bool) {

}
func (c *AddressBookProxySignals) CurrentContactChanged(currentContact addressbook.Contact) {

}
func (c *AddressBookProxySignals) OnContactsChanged(contacts []addressbook.Contact) {
	fmt.Println(contacts)
}
func (c *AddressBookProxySignals) OnReadyChanged(ready bool) {

}

func main() {
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	addressBookSignalHandler := &AddressBookProxySignals{}
	proxy := &addressbook.AddressBookProxy{Conn: conn}
	proxy.Init()
	proxy.ConnectToRemoteObject("goqface.addressbook")
	proxy.AddContactsChangedObserver(addressBookSignalHandler)
	proxy.SetContacts([]addressbook.Contact{addressbook.Contact{1, "JohnDoe", "TelNummer", 2}, addressbook.Contact{2, "MAxMusterman", "Handy", 234}})

	c := make(chan *dbus.Signal, 10)
	for v := range c {
		fmt.Println(v)
	}
}
