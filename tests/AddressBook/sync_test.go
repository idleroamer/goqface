package addressbook

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	goqface "github.com/idleroamer/goqface/objectManager"
	addressbook "github.com/idleroamer/goqface/tests/AddressBook/Tests/AddressBook"
)

//go:generate python3 ../../codegen.py --input AddressBook.qface
//go:generate gofmt -w Tests

type Foo struct {
	Id    int
	Value string
}

type AddressBookImpl struct {
	*addressbook.AddressBookAdapter
}

type AddressBookClient struct {
	wg              *sync.WaitGroup
	contactsChanged int
}

type AddressBookServerObserver struct {
	wg              *sync.WaitGroup
	contactsChanged int
}

var Idx int

func (c *AddressBookImpl) CreateNewContact() *dbus.Error {
	var contact addressbook.Contact
	contact.Idx = Idx
	Idx++
	contact.Name = "Name" + strconv.Itoa(contact.Idx)
	contact.Number = "12345" + strconv.Itoa(contact.Idx)
	contact.Type = addressbook.Family
	c.AssignContacts(append(c.Contacts(), contact))
	fmt.Printf("newContactCreated: %v", len(c.Contacts()))
	c.ContactCreated(contact)
	return nil
}

func (c *AddressBookImpl) SelectContact(contactId int) *dbus.Error {
	found := false
	for _, entry := range c.Contacts() {
		if entry.Idx == contactId {
			if c.CurrentContact().Idx != contactId {
				c.AssignCurrentContact(entry)
				fmt.Printf("SelectContact: %d", contactId)
			} else {
				fmt.Printf("SelectContact already selected: %d", contactId)
			}
			found = true
		}
	}
	if !found {
		return dbus.MakeFailedError(dbus.ErrMsgInvalidArg)
	}
	return nil
}

func (c *AddressBookImpl) DeleteContact(contactId int) (bool, *dbus.Error) {
	found := false
	i := 0
	tmpContacts := c.Contacts()
	for _, entry := range tmpContacts {
		if entry.Idx == contactId {
			toBeDeletedContact := entry
			c.ContactDeleted(toBeDeletedContact)
			fmt.Printf("DeleteContact: %d", contactId)
			found = true
		} else {
			tmpContacts[i] = entry
			i++
		}
	}
	for j := i; j < len(tmpContacts); j++ {
		fmt.Printf("newContactCreated: %v", len(c.Contacts()))
		tmpContacts[j] = addressbook.Contact{}
	}
	if found {
		c.AssignContacts(tmpContacts)
	} else {
		return true, dbus.MakeFailedError(dbus.ErrMsgInvalidArg)
	}
	return true, nil
}

func (c *AddressBookImpl) UpdateContact(contactId int, contact addressbook.Contact) *dbus.Error {
	if contactId >= 0 && contactId < len(c.Contacts()) {
		c.Contacts()[contactId] = contact
		fmt.Printf("UpdateContact: %v", contact)
	} else {
		c.ContactUpdateFailed(addressbook.Other)
	}
	return nil
}

func (c *AddressBookImpl) SetCurrentContact(value addressbook.Contact) error {
	if value.Idx == -1 {
		return errors.New("Wrong value")
	}
	return nil
}

func (c *AddressBookClient) OnContactsChanged(contacts []addressbook.Contact) {
	c.contactsChanged++
	c.wg.Done()
}

func (c *AddressBookClient) OnContactCreated(contacts addressbook.Contact) {
	c.wg.Done()
}

func (c *AddressBookClient) OnReadyChanged(ready bool) {
	c.wg.Done()
}

func (c *AddressBookServerObserver) OnContactsChanged(contacts []addressbook.Contact) {
	c.contactsChanged++
	c.wg.Done()
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

func TestSetProperty(t *testing.T) {
	var wg sync.WaitGroup

	server, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	client, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	addressbookAdapter := &addressbook.AddressBookAdapter{Conn: server}
	addressBookImpl := &AddressBookImpl{addressbookAdapter}
	addressbookAdapter.Init(addressBookImpl)
	defer goqface.ObjectManager(server).UnregisterObject(addressbookAdapter.ObjectPath(), nil)
	addressbookAdapter.Export()

	addressBookServerObserver := &AddressBookServerObserver{wg: &wg}

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookClient := &AddressBookClient{wg: &wg}
	addressBookProxy.Init()
	addressBookProxy.SetServiceName(server.Names()[0])
	addressBookProxy.ConnectToServer()

	contacts := []addressbook.Contact{addressbook.Contact{1, "JohnDoe", "0198349343", addressbook.Friend}, addressbook.Contact{2, "MaxMusterman", "823439343", addressbook.Family}}

	addressBookImpl.AddContactsChangedObserver(addressBookServerObserver)
	addressBookProxy.AddContactsChangedObserver(addressBookClient)
	wg.Add(1)
	wg.Add(1)
	errSetProp := addressBookProxy.SetContacts(contacts)
	if errSetProp != nil {
		t.Errorf("call to remote object failed! %v", errSetProp)
	}

	if waitTimeout(&wg, time.Second) {
		t.Errorf("Timed out waiting for wait group")
	}

	if addressBookServerObserver.contactsChanged != 1 {
		t.Errorf("failed to get %v contactsChanged signal, got %v", 1, addressBookServerObserver.contactsChanged)
	}

	if addressBookClient.contactsChanged != 1 {
		t.Errorf("failed to get %v contactsChanged signal, got %v", 1, addressBookClient.contactsChanged)
	}

	if !reflect.DeepEqual(contacts, addressbookAdapter.Contacts()) {
		t.Errorf("failed to set remote object prop! have %v want %v", addressbookAdapter.Contacts(), contacts)
	}

	otherContacts := []addressbook.Contact{addressbook.Contact{3, "NoName", "NoNumber", addressbook.Family}}

	// wait group will panic if observer not removed due to negative wg counter
	addressBookImpl.RemoveContactsChangedObserver(addressBookServerObserver)
	addressBookProxy.RemoveContactsChangedObserver(addressBookClient)
	errSetPropAgain := addressBookProxy.SetContacts(otherContacts)
	if errSetPropAgain != nil {
		t.Errorf("call to remote object failed! %v", errSetPropAgain)
	}
}

func TestSetPropertyNotAllowed(t *testing.T) {
	server, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	client, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	addressbookAdapter := &addressbook.AddressBookAdapter{Conn: server}
	addressBookImpl := &AddressBookImpl{addressbookAdapter}
	addressbookAdapter.Init(addressBookImpl)
	addressbookAdapter.Export()
	defer goqface.ObjectManager(server).UnregisterObject(addressbookAdapter.ObjectPath(), nil)
	addressbookAdapter.SetCurrentContactCallback(addressBookImpl)

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookProxy.Init()
	addressBookProxy.SetServiceName(server.Names()[0])
	addressBookProxy.ConnectToServer()

	errSetProp := addressBookProxy.SetCurrentContact(addressbook.Contact{Idx: -1})
	if errSetProp.Error() != "Wrong value" {
		t.Errorf("setCurrentContact accepted wrong value")
	}
	errSetProp2 := addressBookProxy.SetCurrentContact(addressbook.Contact{Idx: 1})
	if errSetProp2 != nil {
		t.Errorf("setCurrentContact failed to accept wrong value")
	}

}

func TestCallMethod(t *testing.T) {
	var wg sync.WaitGroup
	server, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	client, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	addressbookAdapter := &addressbook.AddressBookAdapter{Conn: server}
	addressBookImpl := &AddressBookImpl{addressbookAdapter}
	addressbookAdapter.Init(addressBookImpl)
	addressbookAdapter.Export()
	defer goqface.ObjectManager(server).UnregisterObject(addressbookAdapter.ObjectPath(), nil)

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookProxy.Init()
	addressBookProxy.SetServiceName(server.Names()[0])
	addressBookProxy.ConnectToServer()

	addressBookClient := &AddressBookClient{wg: &wg}
	addressBookProxy.AddContactsChangedObserver(addressBookClient)
	wg.Add(1)
	errCallMethod := addressBookProxy.CreateNewContact()
	if errCallMethod != nil {
		t.Errorf("call to remote object failed! %v", errCallMethod)
	}

	if waitTimeout(&wg, time.Second) {
		t.Errorf("Timed out waiting for wait group")
	}

	if !reflect.DeepEqual(addressBookProxy.Contacts(), addressbookAdapter.Contacts()) {
		t.Errorf("Object value mismatch! have %v want %v", addressBookProxy.Contacts(), addressbookAdapter.Contacts())
	}
	// intentionally select a non-existing index
	errCallMethod = addressBookProxy.SelectContact(1)
	if errCallMethod == nil {
		t.Errorf("remote func didn't return error as expected")
	}

	addressBookProxy.RemoveContactsChangedObserver(addressBookClient)
}

func TestSignal(t *testing.T) {
	var wg sync.WaitGroup
	server, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	client, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	addressbookAdapter := &addressbook.AddressBookAdapter{Conn: server}
	addressBookImpl := &AddressBookImpl{addressbookAdapter}
	addressbookAdapter.Init(addressBookImpl)
	addressbookAdapter.Export()
	defer goqface.ObjectManager(server).UnregisterObject(addressbookAdapter.ObjectPath(), nil)

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookProxy.Init()
	addressBookProxy.SetServiceName(server.Names()[0])
	addressBookProxy.ConnectToServer()

	addressBookClient := &AddressBookClient{wg: &wg}
	addressBookProxy.AddContactCreatedObserver(addressBookClient)
	wg.Add(1)
	errCallMethod := addressBookProxy.CreateNewContact()
	if errCallMethod != nil {
		t.Errorf("call to remote object failed! %v", errCallMethod)
	}

	if waitTimeout(&wg, time.Second) {
		t.Errorf("Timed out waiting for wait group")
	}
}

func TestGetAllOnReadyChanged(t *testing.T) {
	var wg sync.WaitGroup
	server, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	client, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	addressbookAdapter := &addressbook.AddressBookAdapter{Conn: server}
	addressBookImpl := &AddressBookImpl{addressbookAdapter}
	addressbookAdapter.Init(addressBookImpl)
	addressbookAdapter.Export()
	defer goqface.ObjectManager(server).UnregisterObject(addressbookAdapter.ObjectPath(), nil)
	addressBookImpl.AssignReady(true)
	intValues := []int{1, 2, 3}
	addressBookImpl.AssignIntValues(intValues)

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookProxy.Init()
	addressBookProxy.SetServiceName(server.Names()[0])

	addressBookClient := &AddressBookClient{wg: &wg}
	addressBookProxy.AddReadyChangedObserver(addressBookClient)
	wg.Add(1)

	addressBookProxy.ConnectToServer()

	if waitTimeout(&wg, time.Second) {
		t.Errorf("Timed out waiting for wait group")
	}
	addressBookProxy.RemoveReadyChangedObserver(addressBookClient)
	if addressBookProxy.Ready() != true {
		t.Errorf("GetAll properties is not called on ConnectToServer!")
	}
	if !reflect.DeepEqual(addressBookProxy.IntValues(), intValues) {
		t.Errorf("GetAll properties is not called on ConnectToServer!")
	}
}

func TestObjectManager(t *testing.T) {
	var wg sync.WaitGroup
	server, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}
	client, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}

	addressbookAdapter := &addressbook.AddressBookAdapter{Conn: server}
	addressBookImpl := &AddressBookImpl{addressbookAdapter}
	addressbookAdapter.Init(addressBookImpl)
	addressbookAdapter.SetObjectPath(addressbookAdapter.ObjectPath() + "/ObjectManagement")
	addressbookAdapter.Export()
	addressBookImpl.AssignReady(true)
	intValues := []int{1, 2, 3}
	addressBookImpl.AssignIntValues(intValues)

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookProxy.Init()
	addressBookProxy.SetObjectPath(addressBookProxy.ObjectPath() + "/ObjectManagement")

	addressBookClient := &AddressBookClient{wg: &wg}
	addressBookProxy.AddReadyChangedObserver(addressBookClient)
	wg.Add(1)

	addressBookProxy.ConnectToServer()

	if waitTimeout(&wg, time.Second) {
		t.Errorf("Timed out waiting for wait group")
	}
	if addressBookProxy.Ready() != true {
		t.Errorf("proxy not connected automatically to adapter!")
	}
	if !reflect.DeepEqual(addressBookProxy.IntValues(), intValues) {
		t.Errorf("GetAll properties is not called on ConnectToServer!")
	}

	wg.Add(1)
	goqface.ObjectManager(server).UnregisterObject(addressbookAdapter.ObjectPath(), nil)
	if waitTimeout(&wg, time.Second) {
		t.Errorf("Timed out waiting for wait group")
	}
	if addressBookProxy.Ready() != false {
		t.Errorf("proxy not automatically informed about adapter object removal!")
	}
}
