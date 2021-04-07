package main

//go:generate python3 ../../codegen.py --input AddressBook.qface
//go:generate gofmt -w Examples

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	addressbook "github.com/idleroamer/goqface/_examples/AddressBook/Examples/AddressBook"

	"github.com/godbus/dbus/v5"
)

type AddressBookImpl struct {
	*addressbook.AddressBookBase
}

var Idx int

func (addressbookInterface *AddressBookImpl) CreateNewContact() *dbus.Error {
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

func (addressbookInterface *AddressBookImpl) SelectContact(contactId int) *dbus.Error {
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

func (addressbookInterface *AddressBookImpl) DeleteContact(contactId int) (bool, *dbus.Error) {
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

func (addressbookInterface *AddressBookImpl) UpdateContact(contactId int, contact addressbook.Contact) *dbus.Error {
	if contactId >= 0 && contactId < len(addressbookInterface.Contacts()) {
		addressbookInterface.Contacts()[contactId] = contact
		fmt.Printf("UpdateContact: %v", contact)
	} else {
		addressbookInterface.ContactUpdateFailed(addressbook.Other)
	}
	return nil
}

func (addressbookInterface *AddressBookImpl) CurrentContactAboutToBeSet(value addressbook.Contact) error {
	return errors.New("No way")
}

func (addressbookInterface *AddressBookImpl) IsLoadedAboutToBeSet(isLoaded bool) error {
	return nil
}

func (addressbookInterface *AddressBookImpl) ContactsAboutToBeSet(contacts []addressbook.Contact) error {
	return nil
}

func (addressbookInterface *AddressBookImpl) IntValuesAboutToBeSet(intValues []int) error {
	return nil
}

func (addressbookInterface *AddressBookImpl) NestedAboutToBeSet(nested addressbook.Nested) error {
	return nil
}

func main() {
	addressbookServiceName := "goqface.addressbook"

	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	reply, err := conn.RequestName(addressbookServiceName, dbus.NameFlagDoNotQueue)
	if err != nil {
		panic(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		fmt.Fprintln(os.Stderr, "name already taken")
		os.Exit(1)
	}

	addressbookAdapter := &addressbook.AddressBookAdapter{Conn: conn}
	addressBookImpl := &AddressBookImpl{&addressbook.AddressBookBase{}}

	addressbookAdapter.Init(addressBookImpl)
	addressbookAdapter.Export()

	fmt.Println("Listening on serviceName: " + addressbookServiceName + " objectPath: " + string(addressbookAdapter.ObjectPath()) + "...")

	c := make(chan *dbus.Signal)
	conn.Signal(c)
	for _ = range c {
	}
}
