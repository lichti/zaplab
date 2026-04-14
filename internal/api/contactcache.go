package api

import (
	"net/http"

	"github.com/lichti/zaplab/internal/whatsapp"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// getContactCache returns cached contacts, optionally filtered by search query.
func getContactCache(e *core.RequestEvent) error {
	type row struct {
		ID          string `db:"id"               json:"id"`
		JID         string `db:"jid"              json:"jid"`
		Name        string `db:"name"             json:"name"`
		Phone       string `db:"phone"            json:"phone"`
		About       string `db:"about"            json:"about"`
		AvatarURL   string `db:"avatar_url"       json:"avatar_url"`
		LastSeen    string `db:"last_seen"        json:"last_seen"`
		CacheUpdAt  string `db:"cache_updated_at" json:"cache_updated_at"`
	}

	q := pb.DB().
		Select("id", "jid", "name", "phone", "about", "avatar_url", "last_seen", "cache_updated_at").
		From("contact_cache").
		OrderBy("name ASC").
		Limit(500)

	if s := e.Request.URL.Query().Get("q"); s != "" {
		q = q.AndWhere(dbx.NewExp(
			"(name LIKE {:q} OR phone LIKE {:q} OR jid LIKE {:q})",
			dbx.Params{"q": "%" + s + "%"},
		))
	}

	var rows []row
	if err := q.All(&rows); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if rows == nil {
		rows = []row{}
	}
	return e.JSON(http.StatusOK, map[string]any{"contacts": rows, "total": len(rows)})
}

// postContactCacheRefresh forces a live fetch and cache update for a JID.
func postContactCacheRefresh(e *core.RequestEvent) error {
	jidStr := e.Request.URL.Query().Get("jid")
	if jidStr == "" {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "jid is required"})
	}
	jid, ok := whatsapp.ParseJID(jidStr)
	if !ok {
		return e.JSON(http.StatusBadRequest, map[string]any{"error": "invalid jid"})
	}
	info, err := whatsapp.GetContactInfo(jid)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, info)
}

// deleteContactCacheEntry removes a single JID from the cache.
func deleteContactCacheEntry(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := pb.FindRecordById("contact_cache", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	if err := pb.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"deleted": true})
}

// postContactCachePopulate runs a background job to populate the cache for all known contacts.
func postContactCachePopulate(e *core.RequestEvent) error {
	go func() {
		contacts, err := whatsapp.GetAllContacts()
		if err != nil {
			return
		}
		for _, c := range contacts {
			jid, ok := whatsapp.ParseJID(c.JID)
			if !ok {
				continue
			}
			whatsapp.GetContactInfo(jid) //nolint:errcheck — triggers cache upsert
		}
	}()
	return e.JSON(http.StatusOK, map[string]any{"message": "cache population started in background"})
}
