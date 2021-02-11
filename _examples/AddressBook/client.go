package main

//go:generate python3 ../../codegen.py --input AddressBook.qface
//go:generate gofmt -w Examples

import (
	"fmt"

	addressbook "github.com/idleroamer/goqface/_examples/AddressBook/Examples/AddressBook"

	"github.com/godbus/dbus/v5"
)

type AddressBookProxyImpl struct {
	*addressbook.AddressBookProxy
}

func (c *AddressBookProxyImpl) OnContactCreated(contact addressbook.Contact) {

}
func (c *AddressBookProxyImpl) OnContactUpdateFailed(failureReason addressbook.FailureReason) {

}
func (c *AddressBookProxyImpl) OnContactDeleted(contact addressbook.Contact) {

}
func (c *AddressBookProxyImpl) OnContactUpdatedTo(index int, contact addressbook.Contact) {

}
func (c *AddressBookProxyImpl) IsLoadedChanged(isLoaded bool) {

}
func (c *AddressBookProxyImpl) CurrentContactChanged(currentContact addressbook.Contact) {

}
func (c *AddressBookProxyImpl) ContactsChanged(contacts []addressbook.Contact) {

}
func (c *AddressBookProxyImpl) IntValuesChanged(intValues []int) {

}
func (c *AddressBookProxyImpl) MapOfContactsChanged(mapOfC map[string]addressbook.Contact) {

}
func (c *AddressBookProxyImpl) NestedChanged(nested addressbook.Nested) {

}
func (c *AddressBookProxyImpl) ReadyChanged(ready bool) {

}

func main() {
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	proxy := &AddressBookProxyImpl{&addressbook.AddressBookProxy{Conn: conn}}
	proxy.Init(proxy)
	proxy.ConnectToServer("goqface.addressbook")
	proxy.Setcontacts([]addressbook.Contact{addressbook.Contact{1, "JohnDoe", "TelNummer", 2}, addressbook.Contact{2, "MAxMusterman", "Handy", 234}})

	c := make(chan *dbus.Signal, 10)
	for v := range c {
		fmt.Println(v)
	}
}
