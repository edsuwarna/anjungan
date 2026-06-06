# Anjungan — Bookmarks Implementation Plan

> **Goal:** Implement the Bookmarks feature (CRUD + Dashboard Widget + Sidebar Quick Access + Dedicated Page)
> **PRD Reference:** `prd/PRD-bookmarks.md`
> **Priority:** P0 (easy + high impact)
> **Estimated Effort:** ~4.5 days total

---

## Phase 1 — Backend Core (Day 1)

### Step 1.1 — DB Migration: `000020_create_bookmarks`

**Files:** `backend/migrations/000020_create_bookmarks.up.sql`, `.down.sql`

SQL (from PRD Section 4):

```sql
CREATE TABLE bookmarks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    url         TEXT NOT NULL,
    icon_type   TEXT NOT NULL DEFAULT 'auto'
                CHECK (icon_type IN ('auto', 'iconify', 'emoji')),
    icon_value  TEXT,
    category    TEXT NOT NULL DEFAULT 'Other'
                CHECK (category IN (
                    'Monitoring', 'CI/CD', 'Logging',
                    'Code & Registry', 'Internal Tools', 'Other'
                )),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bookmarks_user_id ON bookmarks(user_id);
CREATE INDEX idx_bookmarks_sort ON bookmarks(user_id, sort_order);
```

Down migration: `DROP TABLE IF EXISTS bookmarks;`

### Step 1.2 — Model: `model.Bookmark`

**File:** `backend/internal/common/model/model.go`

Add structs:
- `Bookmark` — full DB model (includes user_id, icon_type, icon_value, category, sort_order, timestamps)
- `CreateBookmarkRequest` — input for POST
- `UpdateBookmarkRequest` — input for PUT (pointer fields for partial update)
- `BookmarkResponse` — public-safe response (omit user_id)
- `ReorderInput` — `{Id string, SortOrder int}` for PATCH reorder

URL validation: reject `javascript:`, `file:`, `data:` protocols in the model/handler.

### Step 1.3 — Repository: `db.Repository` methods

**File:** `backend/internal/common/db/repository.go` (add at end)

Methods to add:
- `CreateBookmark(ctx, b *model.Bookmark) error`
- `GetBookmarkByID(ctx, id string) (*model.Bookmark, error)`
- `ListBookmarks(ctx, userID string) ([]*model.Bookmark, error)` — sorted by `sort_order ASC`
- `ListBookmarksWithLimit(ctx, userID string, limit int) ([]*model.Bookmark, error)` — for sidebar/dashboard
- `UpdateBookmark(ctx, b *model.Bookmark) error`
- `DeleteBookmark(ctx, id, userID string) error` — owner check in WHERE
- `ReorderBookmarks(ctx, userID string, items []model.ReorderInput) error` — batch update in transaction

### Step 1.4 — Handler: `internal/bookmarks/handler.go`

**New package:** `backend/internal/bookmarks/`

Pattern follows `internal/infra/sshkey_handler.go`:
- `type Handler struct { repo *db.Repository }`
- `func NewHandler(repo *db.Repository) *Handler`
- `func (h *Handler) Routes() chi.Router` — mounts under `/api/v1/bookmarks`

Routes:
| Method | Path | Auth | Handler | Description |
|--------|------|------|---------|-------------|
| GET | `/` | JWT | `h.List` | List user's bookmarks. Optional: `?limit=N`, `?category=X`, `?q=search` |
| POST | `/` | JWT | `h.Create` | Create bookmark. Body: `{title, url, icon_type?, icon_value?, category?, sort_order?}` |
| GET | `/{id}` | JWT | `h.Get` | Get single bookmark (owner check) |
| PUT | `/{id}` | JWT | `h.Update` | Update bookmark (owner check) |
| DELETE | `/{id}` | JWT | `h.Delete` | Delete bookmark (owner check) |
| PATCH | `/reorder` | JWT | `h.Reorder` | Bulk reorder. Body: `[{id, sort_order}, ...]` |

Handler details:
- `List`: Get `userID` from JWT claims. Query DB, return list. Support `?limit=` for truncation, `?q=` for frontend-side filtering hint.
- `Create`: Decode JSON, validate title+url required. Auto-prepend `https://` if no protocol. Sanitize URL (reject dangerous protocols). Set `userID` from JWT claims. Default `category` to "Other". Default `icon_type` to "auto". If no `sort_order`, set to `max(sort_order) + 1`. Audit log.
- `Get`: URL param `{id}`, fetch by ID, verify `user_id == claims.UserID` (or return 404 to avoid leaking existence).
- `Update`: Decode partial JSON. Fetch existing, verify owner. Update non-nil fields. Audit log.
- `Delete`: URL param `{id}`, verify owner. Delete. Audit log.
- `Reorder`: Decode `[{id, sort_order}]`. Verify all IDs belong to user. Batch update in transaction. Audit log.

All responses use `common.JSON()` wrapper.

### Step 1.5 — Wire into router

**File:** `backend/internal/server/server.go`

1. Add import: `"github.com/edsuwarna/anjungan/internal/bookmarks"`
2. In `setupRouter`, add inside the protected `/api/v1` group:
   ```go
   r.Mount("/bookmarks", bookmarks.NewHandler(repo).Routes())
   ```

### Step 1.6 — API Client

**File:** `frontend/src/lib/api.svelte.js`

Add `bookmarks` namespace:

```js
bookmarks: {
    list: (params) => {
        const q = params ? '?' + new URLSearchParams(params).toString() : '';
        return request(`/bookmarks${q}`);
    },
    get: (id) => request(`/bookmarks/${id}`),
    create: (data) => request('/bookmarks', { method: 'POST', body: JSON.stringify(data) }),
    update: (id, data) => request(`/bookmarks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id) => request(`/bookmarks/${id}`, { method: 'DELETE' }),
    reorder: (items) => request('/bookmarks/reorder', { method: 'PATCH', body: JSON.stringify(items) }),
},
```

---

## Phase 2 — Frontend: Bookmarks Page (Day 2-3)

### Step 2.1 — Route: `src/routes/bookmarks/+page.svelte`

**New directory:** `frontend/src/routes/bookmarks/`

Page layout:
- **Header**: Title "Bookmarks" + "Add Bookmark" button (primary)
- **Search bar** + **category filter chips**: All, Monitoring, CI/CD, Logging, Code & Registry, Internal Tools, Other
- **Card grid** (responsive: 4→2→1 cols)
- Each card: icon/favicon (40×40), tool name (bold), URL (truncated), category pill badge
- Click card → open URL in new tab
- **States**: loading (skeleton grid), empty ("🔖 No bookmarks yet"), error (toast), data

### Step 2.2 — Add/Edit Modal: `BookmarkFormModal.svelte`

**New file:** `frontend/src/lib/components/bookmarks/BookmarkFormModal.svelte`

Fields:
- Tool Name (text, required, max 100)
- URL (text, required, auto-prepend `https://`)
- Category (dropdown: Monitoring, CI/CD, Logging, Code & Registry, Internal Tools, Other)
- Icon (auto-favicon preview on URL blur + emoji fallback)
- Modal: max-width 480px, centered

### Step 2.3 — Sidebar Quick Access

**File:** `frontend/src/lib/components/layout/Sidebar.svelte`

Add collapsible "⚡ Quick Access" section in the nav (between Ops and Security categories, or at bottom before dark mode toggle). Fetches first 5 bookmarks. Shows compact icon + label. Click → open new tab.

### Step 2.4 — Dashboard Widget "Your Tools"

**File:** `frontend/src/routes/+page.svelte`

Add widget after stat cards (before Server Status chart):
- Title "🔖 Your Tools" with "Manage →" link to `/bookmarks`
- Grid of 8 bookmark items: icon (24×24) + name
- Compact row layout, hoverable
- Empty state: inline "Add Bookmark" button

---

## Phase 3 — Polish & Edge Cases (Day 4)

### Step 3.1 — Auto-Favicon

On URL blur in modal, attempt to show favicon using `https://www.google.com/s2/favicons?domain=...`. If fails, fallback to first letter of tool name in a colored circle.

### Step 3.2 — Drag-to-Reorder

On `/bookmarks` page, add drag handle (⠿) on hover. HTML5 Drag & Drop API. Debounced PATCH call on drop (500ms). If API fails, snap back + error toast.

### Step 3.3 — Validation & Edge Cases

- URL without protocol → auto-prepend `https://`
- Reject `javascript:`, `file:`, `data:` URLs
- Long title → truncate with ellipsis (2 lines)
- Long URL → single-line truncate
- Mobile: responsive grid, sidebar collapse
- Dark mode: all card/text/border colors

---

## Files Changed Summary

### Backend (Go)
| File | Action | Description |
|------|--------|-------------|
| `backend/migrations/000020_create_bookmarks.up.sql` | **Create** | Bookmarks table DDL |
| `backend/migrations/000020_create_bookmarks.down.sql` | **Create** | Drop table |
| `backend/internal/common/model/model.go` | **Edit** | Add Bookmark structs |
| `backend/internal/common/db/repository.go` | **Edit** | Add Bookmark repo methods |
| `backend/internal/bookmarks/handler.go` | **Create** | Bookmark HTTP handler |
| `backend/internal/server/server.go` | **Edit** | Wire /bookmarks route |

### Frontend (SvelteKit)
| File | Action | Description |
|------|--------|-------------|
| `frontend/src/lib/api.svelte.js` | **Edit** | Add bookmarks API methods |
| `frontend/src/routes/bookmarks/+page.svelte` | **Create** | Bookmarks management page |
| `frontend/src/lib/components/bookmarks/BookmarkFormModal.svelte` | **Create** | Add/edit modal |
| `frontend/src/lib/components/layout/Sidebar.svelte` | **Edit** | Add Quick Access section |
| `frontend/src/routes/+page.svelte` | **Edit** | Add Your Tools widget |

---

## Verification

1. **Backend**: Run `go build ./...` — should compile clean
2. **Migration**: Run app — should auto-apply migration 000020
3. **API**: Test with curl:
   - `POST /api/v1/bookmarks` → create bookmark
   - `GET /api/v1/bookmarks` → list
   - `PUT /api/v1/bookmarks/{id}` → update
   - `DELETE /api/v1/bookmarks/{id}` → delete
   - `PATCH /api/v1/bookmarks/reorder` → reorder
4. **Frontend**: Open `/bookmarks` → should load list
5. **Dashboard**: Homepage → "Your Tools" widget visible
6. **Sidebar**: "Quick Access" section visible with first 5 bookmarks

---

## Risks & Notes

- **No favicon service dependency**: Google favicon API is best-effort. If it fails, fallback gracefully.
- **No pagination needed for Phase 1**: Bookmarks are per-user, typically < 50. Can add server-side pagination in Phase 2.
- **URL safety**: Critical to sanitize URLs at the backend level. The `SaveActivity` / audit log pattern already exists and should be reused.
- **Existing patterns**: The SSH key handler (`infra/sshkey_handler.go`) is the closest pattern to follow — simple CRUD with audit logging.
- **Frontend Svelte 5 runes**: Use `$state`, `$derived`, `$effect` patterns already established in the codebase.
