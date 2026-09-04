package webdb_test

import (
	"monorepo/twigg-web/education"
	"monorepo/twigg-web/webdb"
	"reflect"
	"testing"
)

func TestGetUserEducationWithoutRowReturnsDefault(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	r, closeR, err := b.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	defer closeR()

	const userId = 10 // userId with no row
	got, err := b.GetUserEducation(r, userId)
	if err != nil {
		t.Fatal(err)
	}

	expected := education.UserEducation{
		UserId:          userId,
		WelcomeWasShown: false,
	}
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("unexpected user education\nexpected: %#v\ngot: %#v", expected, got)
	}
}

func TestSetUserEducationCreatesMissingRow(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	const userId = 10 // userId with no row

	ed := education.UserEducation{
		UserId:          userId,
		WelcomeWasShown: true,
	}
	err = b.SetUserEducation(w, ed)
	if err != nil {
		t.Fatal(err)
	}

	got, err := b.GetUserEducation(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if !got.WelcomeWasShown {
		t.Fatal("expected the row to be created with WelcomeWasShown true")
	}
}

func TestSetUserEducationOverwritesExistingRow(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	const userId = 10

	ed := education.UserEducation{
		UserId:          userId,
		WelcomeWasShown: true,
	}
	err = b.SetUserEducation(w, ed)
	if err != nil {
		t.Fatal(err)
	}

	ed.WelcomeWasShown = false
	err = b.SetUserEducation(w, ed)
	if err != nil {
		t.Fatal(err)
	}

	got, err := b.GetUserEducation(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if got.WelcomeWasShown {
		t.Fatal("expected WelcomeWasShown to be overwritten to false")
	}
}

func TestSetWelcomeWasShownAndGetUserEducation(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	const userId = 10
	const otherUserId = 999

	err = b.SetWelcomeWasShown(w, userId, true)
	if err != nil {
		t.Fatal(err)
	}

	got, err := b.GetUserEducation(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	expected := education.UserEducation{
		UserId:          userId,
		WelcomeWasShown: true,
	}
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("unexpected user education\nexpected: %#v\ngot: %#v", expected, got)
	}

	// Other users must not be affected
	gotOther, err := b.GetUserEducation(w, otherUserId)
	if err != nil {
		t.Fatal(err)
	}
	if gotOther.WelcomeWasShown {
		t.Fatal("other user's education should keep the defaults")
	}
}

func TestSetWelcomeWasShownOverwrites(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	const userId = 10

	err = b.SetWelcomeWasShown(w, userId, true)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetWelcomeWasShown(w, userId, false)
	if err != nil {
		t.Fatal(err)
	}

	got, err := b.GetUserEducation(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if got.WelcomeWasShown {
		t.Fatal("expected WelcomeWasShown to be overwritten to false")
	}
}