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

type AddressBookProxyObserver struct {
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

func (c AddressBookImpl) IsLoadedAboutToBeSet(value bool) error {

	return nil
}

func (c *AddressBookImpl) CurrentContactAboutToBeSet(value addressbook.Contact) error {
	return errors.New("No way")
}

func (c *AddressBookImpl) ContactsAboutToBeSet(contacts []addressbook.Contact) error {
	return nil
}

func (c *AddressBookImpl) IntValuesAboutToBeSet(intValues []int) error {
	return nil
}

func (c *AddressBookImpl) NestedAboutToBeSet(nested addressbook.Nested) error {
	return nil
}

func (c *AddressBookProxyObserver) OnContactCreated(contact addressbook.Contact) {

}
func (c *AddressBookProxyObserver) OnContactUpdateFailed(failureReason addressbook.FailureReason) {

}
func (c *AddressBookProxyObserver) OnContactDeleted(contact addressbook.Contact) {

}
func (c *AddressBookProxyObserver) OnContactUpdatedTo(index int, contact addressbook.Contact) {

}
func (c *AddressBookProxyObserver) OnContactsChanged(contacts []addressbook.Contact) {
	fmt.Println("OnContactsChanged")
	c.contactsChanged++
	c.wg.Done()
}
func (c *AddressBookProxyObserver) IsLoadedChanged(isLoaded bool) {

}
func (c *AddressBookProxyObserver) CurrentContactChanged(currentContact addressbook.Contact) {

}
func (c *AddressBookProxyObserver) ContactsChanged(contacts []addressbook.Contact) {

}
func (c *AddressBookProxyObserver) IntValuesChanged(intValues []int) {

}
func (c *AddressBookProxyObserver) MapOfContactsChanged(mapOfC map[string]addressbook.Contact) {

}
func (c *AddressBookProxyObserver) NestedChanged(nested addressbook.Nested) {

}
func (c *AddressBookProxyObserver) ReadyChanged(ready bool) {

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
	addressbookAdapter.Export()

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookProxyObserver := &AddressBookProxyObserver{wg: &wg}
	addressBookProxy.Init()
	addressBookProxy.ConnectToServer(server.Names()[0])

	contacts := []addressbook.Contact{addressbook.Contact{1, "JohnDoe", "0198349343", addressbook.Friend}, addressbook.Contact{2, "MaxMusterman", "823439343", addressbook.Family}}

	addressBookProxy.AddContactsChangedObserver(addressBookProxyObserver)
	wg.Add(1)
	errSetProp := addressBookProxy.Setcontacts(contacts)
	if errSetProp != nil {
		t.Errorf("call to remote object failed! %v", errSetProp)
	}

	if waitTimeout(&wg, time.Second) {
		t.Errorf("Timed out waiting for wait group")
	}

	if addressBookProxyObserver.contactsChanged != 1 {
		t.Errorf("failed to get %v contactsChanged signal, got %v", 1, addressBookProxyObserver.contactsChanged)
	}

	if !reflect.DeepEqual(contacts, addressbookAdapter.Contacts()) {
		t.Errorf("failed to set remote object prop! have %v want %v", addressbookAdapter.Contacts(), contacts)
	}

	otherContacts := []addressbook.Contact{addressbook.Contact{3, "NoName", "NoNumber", addressbook.Family}}

	// wait group will panic if observer not removed due to negative wg counter
	addressBookProxy.RemoveContactsChangedObserver(addressBookProxyObserver)
	errSetPropAgain := addressBookProxy.Setcontacts(otherContacts)
	if errSetPropAgain != nil {
		t.Errorf("call to remote object failed! %v", errSetPropAgain)
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

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookProxy.Init()
	addressBookProxy.ConnectToServer(server.Names()[0])

	addressBookProxyObserver := &AddressBookProxyObserver{wg: &wg}
	addressBookProxy.AddContactsChangedObserver(addressBookProxyObserver)
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

	addressBookProxy.RemoveContactsChangedObserver(addressBookProxyObserver)

}
