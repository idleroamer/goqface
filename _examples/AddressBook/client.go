package main

//go:generate python3 ../../codegen.py --input AddressBook.qface
//go:generate gofmt -w Examples

import (
	"fmt"

	addressbook "github.com/goqface/_examples/AddressBook/Examples/AddressBook"

	"github.com/godbus/dbus/v5"
)

type AddressBookSignalListener struct {
}

func (c *AddressBookSignalListener) OnContactCreated(contact addressbook.Contact) {

}
func (c *AddressBookSignalListener) OnContactUpdateFailed(failureReason addressbook.FailureReason) {

}
func (c *AddressBookSignalListener) OnContactDeleted(contact addressbook.Contact) {

}
func (c *AddressBookSignalListener) OnContactUpdatedTo(index int, contact addressbook.Contact) {

}
func (c *AddressBookSignalListener) IsLoadedChanged(isLoaded bool) {

}
func (c *AddressBookSignalListener) CurrentContactChanged(currentContact addressbook.Contact) {

}
func (c *AddressBookSignalListener) ContactsChanged(contacts []addressbook.Contact) {

}
func (c *AddressBookSignalListener) IntValuesChanged(intValues []int) {

}

func main() {
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	signalListener := &AddressBookSignalListener{}
	proxy := &addressbook.AddressBookProxy{Conn: conn, SignalListener: signalListener}
	proxy.Init()
	proxy.ConnectToServer("goqface.addressbook")
	proxy.SetContacts([]addressbook.Contact{addressbook.Contact{1, "JohnDoe", "TelNummer", 2}, addressbook.Contact{2, "MAxMusterman", "Handy", 234}})

	c := make(chan *dbus.Signal, 10)
	for v := range c {
		fmt.Println(v)
	}
}
