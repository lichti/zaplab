package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ContactEntry is a contact from the local whatsmeow store.
type ContactEntry struct {
	JID          string `json:"jid"`
	Phone        string `json:"phone"`
	FullName     string `json:"full_name"`
	PushName     string `json:"push_name"`
	BusinessName string `json:"business_name"`
}

// CheckResult is the result for a single phone number lookup.
type CheckResult struct {
	Query        string `json:"query"`
	IsOnWhatsApp bool   `json:"is_on_whatsapp"`
	JID          string `json:"jid,omitempty"`
	VerifiedName string `json:"verified_name,omitempty"`
}

// ContactInfo holds detailed information about a WhatsApp contact.
type ContactInfo struct {
	JID          string `json:"jid"`
	Phone        string `json:"phone"`
	FullName     string `json:"full_name"`
	PushName     string `json:"push_name"`
	BusinessName string `json:"business_name"`
	Status       string `json:"status"`
	AvatarURL    string `json:"avatar_url"`
}

// GetAllContacts returns all contacts stored locally by whatsmeow.
func GetAllContacts() ([]ContactEntry, error) {
	if client == nil || client.Store == nil {
		return nil, fmt.Errorf("not connected")
	}
	contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]ContactEntry, 0, len(contacts))
	for jid, info := range contacts {
		result = append(result, ContactEntry{
			JID:          jid.String(),
			Phone:        jid.User,
			FullName:     info.FullName,
			PushName:     info.PushName,
			BusinessName: info.BusinessName,
		})
	}
	return result, nil
}

// CheckOnWhatsApp checks if the given phone numbers are registered on WhatsApp.
func CheckOnWhatsApp(phones []string) ([]CheckResult, error) {
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}
	normalized := make([]string, 0, len(phones))
	for _, p := range phones {
		p = strings.TrimPrefix(strings.TrimSpace(p), "+")
		if p != "" {
			normalized = append(normalized, p)
		}
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("no valid phone numbers provided")
	}
	resp, err := client.IsOnWhatsApp(context.Background(), normalized)
	if err != nil {
		return nil, err
	}
	results := make([]CheckResult, len(resp))
	for i, r := range resp {
		res := CheckResult{
			Query:        r.Query,
			IsOnWhatsApp: r.IsIn,
		}
		if r.IsIn {
			res.JID = r.JID.String()
			if r.VerifiedName != nil && r.VerifiedName.Details != nil {
				res.VerifiedName = r.VerifiedName.Details.GetVerifiedName()
			}
		}
		results[i] = res
	}
	return results, nil
}

// GetContactInfo fetches user info and profile picture for a given JID.
func GetContactInfo(jid types.JID) (*ContactInfo, error) {
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}
	info := &ContactInfo{
		JID:   jid.String(),
		Phone: jid.User,
	}
	// Push name + full name from local store
	contact, err := client.Store.Contacts.GetContact(context.Background(), jid)
	if err == nil {
		info.FullName = contact.FullName
		info.PushName = contact.PushName
		info.BusinessName = contact.BusinessName
	}
	// Status from server
	userInfoMap, err := client.GetUserInfo(context.Background(), []types.JID{jid})
	if err == nil {
		if ui, ok := userInfoMap[jid]; ok {
			info.Status = ui.Status
			if ui.VerifiedName != nil && ui.VerifiedName.Details != nil && info.BusinessName == "" {
				info.BusinessName = ui.VerifiedName.Details.GetVerifiedName()
			}
		}
	} else {
		logger.Debugf("GetContactInfo: GetUserInfo failed: %v", err)
	}
	// Profile picture
	pic, err := client.GetProfilePictureInfo(context.Background(), jid, &whatsmeow.GetProfilePictureParams{Preview: false})
	if err == nil && pic != nil {
		info.AvatarURL = pic.URL
	} else if err != nil &&
		!errors.Is(err, whatsmeow.ErrProfilePictureNotSet) &&
		!errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) {
		logger.Debugf("GetContactInfo: GetProfilePictureInfo failed: %v", err)
	}
	// Persist to contact_cache asynchronously so callers are not blocked.
	go upsertContactCache(info)
	return info, nil
}

// upsertContactCache writes a ContactInfo snapshot to the contact_cache collection.
// Uses INSERT OR REPLACE on the jid unique index.
func upsertContactCache(info *ContactInfo) {
	if pb == nil || pb.DB() == nil || shuttingDown.Load() {
		return
	}
	name := info.FullName
	if name == "" {
		name = info.PushName
	}
	if name == "" {
		name = info.BusinessName
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05.000Z")
	_, err := pb.DB().NewQuery(
		"INSERT INTO contact_cache (id, jid, name, phone, about, avatar_url, cache_updated_at, created, updated)" +
			" VALUES ({:id}, {:jid}, {:name}, {:phone}, {:about}, {:avatar}, {:upd}, {:upd}, {:upd})" +
			" ON CONFLICT(jid) DO UPDATE SET name=excluded.name, phone=excluded.phone," +
			" about=excluded.about, avatar_url=excluded.avatar_url, cache_updated_at=excluded.cache_updated_at," +
			" updated=excluded.updated",
	).Bind(dbx.Params{
		"id":     fmt.Sprintf("%x", time.Now().UnixNano()),
		"jid":    info.JID,
		"name":   name,
		"phone":  info.Phone,
		"about":  info.Status,
		"avatar": info.AvatarURL,
		"upd":    now,
	}).Execute()
	if err != nil && !shuttingDown.Load() {
		logger.Debugf("upsertContactCache: %v", err)
	}
}

// contactCacheLookup returns a cached contact name for a JID, or empty string if not cached.
func contactCacheLookup(jid string) string {
	if pb == nil || pb.DB() == nil {
		return ""
	}
	type row struct {
		Name string `db:"name"`
	}
	var r row
	_ = pb.DB().Select("name").From("contact_cache").
		Where(dbx.HashExp{"jid": jid}).One(&r)
	_ = strings.Contains // keep import used
	return r.Name
}
