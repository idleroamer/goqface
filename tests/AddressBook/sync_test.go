package addressbook

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/godbus/dbus/v5"
	addressbook "github.com/idleroamer/goqface/tests/AddressBook/Tests/AddressBook"
)

//go:generate python3 ../../codegen.py --input AddressBook.qface
//go:generate gofmt -w Tests

type Foo struct {
	Id    int
	Value string
}

type AddressBookAdapter struct {
	*addressbook.AddressBook
}

var Idx int

func (addressbookInterface *AddressBookAdapter) CreateNewContact() *dbus.Error {
	var contact addressbook.Contact
	contact.Idx = Idx
	Idx++
	contact.Name = "Name" + strconv.Itoa(contact.Idx)
	contact.Number = "12345" + strconv.Itoa(contact.Idx)
	contact.Type = addressbook.Family
	addressbookInterface.AssignContacts(append(addressbookInterface.Contacts(), contact))
	fmt.Printf("newContactCreated: %v", len(addressbookInterface.Contacts()))
	addressbookInterface.ContactCreated(contact)
	return nil
}

func (addressbookInterface *AddressBookAdapter) SelectContact(contactId int) *dbus.Error {
	found := false
	for _, entry := range addressbookInterface.Contacts() {
		if entry.Idx == contactId {
			if addressbookInterface.CurrentContact().Idx != contactId {
				addressbookInterface.AssignCurrentContact(entry)
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

func (addressbookInterface *AddressBookAdapter) DeleteContact(contactId int) (bool, *dbus.Error) {
	found := false
	i := 0
	tmpContacts := addressbookInterface.Contacts()
	for _, entry := range tmpContacts {
		if entry.Idx == contactId {
			toBeDeletedContact := entry
			addressbookInterface.ContactDeleted(toBeDeletedContact)
			fmt.Printf("DeleteContact: %d", contactId)
			found = true
		} else {
			tmpContacts[i] = entry
			i++
		}
	}
	for j := i; j < len(tmpContacts); j++ {
		fmt.Printf("newContactCreated: %v", len(addressbookInterface.Contacts()))
		tmpContacts[j] = addressbook.Contact{}
	}
	if found {
		addressbookInterface.AssignContacts(tmpContacts)
	} else {
		return true, dbus.MakeFailedError(dbus.ErrMsgInvalidArg)
	}
	return true, nil
}

func (addressbookInterface *AddressBookAdapter) UpdateContact(contactId int, contact addressbook.Contact) *dbus.Error {
	if contactId >= 0 && contactId < len(addressbookInterface.Contacts()) {
		addressbookInterface.Contacts()[contactId] = contact
		fmt.Printf("UpdateContact: %v", contact)
	} else {
		addressbookInterface.ContactUpdateFailed(addressbook.Other)
	}
	return nil
}

func (addressbookInterface AddressBookAdapter) IsLoadedAboutToBeSet(value bool) error {

	return nil
}

func (addressbookInterface *AddressBookAdapter) CurrentContactAboutToBeSet(value addressbook.Contact) error {
	return errors.New("No way")
}
func TestValidateStructsAsProp(t *testing.T) {
	server, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	client, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	addressbookAdapter := &AddressBookAdapter{&addressbook.AddressBook{Conn: server}}

	addressbookAdapter.Init(addressbookAdapter)
	addressbookAdapter.Export()

	addressBookProxy := &addressbook.AddressBookProxy{Conn: client}
	addressBookProxy.Init()
	addressBookProxy.ConnectToServer(server.Names()[0])

	contacts := []addressbook.Contact{addressbook.Contact{1, "JohnDoe", "0198349343", addressbook.Friend}, addressbook.Contact{2, "MaxMusterman", "823439343", addressbook.Family}}
	errSetProp := addressBookProxy.SetContacts(contacts)
	if errSetProp != nil {
		t.Errorf("call to remote object failed! %v", errSetProp)
	}

	if !reflect.DeepEqual(contacts, addressbookAdapter.Contacts()) {
		t.Errorf("failed to set remote object prop! have %v want %v", addressbookAdapter.Contacts(), contacts)
	}
}
