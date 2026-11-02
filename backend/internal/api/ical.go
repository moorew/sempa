package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/calsync"
	"github.com/clevercode/sempa/internal/db"
)

type icalHandler struct {
	store      *db.ICalStore
	fmCalStore *db.FastmailCalStore
	configs    *db.IntegrationConfigStore
	users      *db.UserStore
}

// seesInstanceCalendar reports whether the caller may see the instance-wide
// Fastmail calendar, which is the connecting admin's own account. Members see
// only the feeds they subscribed to themselves.
func (h *icalHandler) seesInstanceCalendar(r *http.Request) bool {
	owner := ownerID(r)
	if owner == db.SystemScope {
		return true
	}
	if h.users == nil {
		return false
	}
	u, err := h.users.GetByID(r.Context(), owner)
	return err == nil && u.IsAdmin
}

func (h *icalHandler) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs, err := h.store.ListSubscriptions(r.Context(), ownerID(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list subscriptions")
		return
	}
	respond(w, http.StatusOK, subs)
}

func (h *icalHandler) createSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Color string `json:"color"`
	}
	if err := decode(r, &req); err != nil || req.URL == "" {
		respondError(w, http.StatusBadRequest, "name and url are required")
		return
	}
	if req.Color == "" {
		req.Color = "#6b7280"
	}
	sub, err := h.store.CreateSubscription(r.Context(), uuid.New().String(), req.Name, req.URL, req.Color, ownerID(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}
	// Sync in background so the HTTP response is fast
	go h.syncSubscriptionByID(context.Background(), sub.ID, db.SystemScope)
	respond(w, http.StatusCreated, sub)
}

func (h *icalHandler) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteSubscription(r.Context(), chi.URLParam(r, "id"), ownerID(r)); err != nil {
		respondError(w, http.StatusNotFound, "subscription not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *icalHandler) syncSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.syncSubscriptionByID(r.Context(), id, ownerID(r))
	respond(w, http.StatusOK, map[string]string{"status": "synced"})
}

func (h *icalHandler) listEventsForDate(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		respondError(w, http.StatusBadRequest, "date is required")
		return
	}
	events, err := h.store.ListEventsForDate(r.Context(), date, ownerID(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []db.ICalEvent{}
	}

	// Tag each subscription event with its feed's display name so the client
	// can group/show/hide by calendar.
	if subs, err := h.store.ListSubscriptions(r.Context(), ownerID(r)); err == nil {
		names := make(map[string]string, len(subs))
		for _, s := range subs {
			names[s.ID] = s.Name
		}
		for i := range events {
			events[i].Calendar = names[events[i].SubscriptionID]
		}
	}

	// Merge Fastmail calendar events when the integration is enabled. Each
	// Fastmail calendar becomes its own selectable "calendar" via a stable
	// subscription_id of fastmail:<calendar name>.
	if h.fmCalStore != nil && h.configs != nil && h.seesInstanceCalendar(r) {
		if cfg, err := h.configs.Get(r.Context(), "fastmail_calendar"); err == nil && cfg.Enabled {
			fmEvents, _ := h.fmCalStore.ListEventsForDate(r.Context(), date)
			for _, ev := range fmEvents {
				calName := ev.CalendarName
				if calName == "" {
					calName = "Fastmail"
				}
				events = append(events, db.ICalEvent{
					ID:             "fm:" + ev.ID,
					SubscriptionID: "fastmail:" + calName,
					Calendar:       calName,
					UID:            ev.UID,
					Summary:        ev.Summary,
					Description:    ev.Description,
					Location:       ev.Location,
					StartTime:      ev.StartTime,
					EndTime:        ev.EndTime,
					AllDay:         ev.AllDay,
					Color:          ev.Color,
				})
			}
		}
	}

	respond(w, http.StatusOK, events)
}

func (h *icalHandler) syncSubscriptionByID(ctx context.Context, id, ownerID string) {
	subs, err := h.store.ListSubscriptions(ctx, ownerID)
	if err != nil {
		return
	}
	for _, sub := range subs {
		if sub.ID == id {
			h.syncOne(ctx, sub)
			return
		}
	}
}

func (h *icalHandler) syncOne(ctx context.Context, sub db.ICalSubscription) {
	_ = calsync.SyncICalSubscription(ctx, h.store, sub)
}
