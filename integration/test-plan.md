# Integration Test Plan for Clinic Service

## Context

The clinic service has ~85 HTTP endpoints but integration tests only cover a subset of them through ordered workflow-style tests in `/integration/`. Many endpoints — particularly around clinician invites, patient tags, sites, settings, patient reviews, clinic user details, patient search, and various CRUD error paths — have no integration test coverage. This plan adds test coverage for every untested endpoint and important logic branch, following the existing Ginkgo + Ordered test pattern with HTTP stubs for external services.

## Existing Coverage (will NOT duplicate)

Already covered in `/integration/`:
- **deletions_test.go**: Clinic/clinician/patient create + delete lifecycle, deletion records
- **migration_test.go**: EnableNewClinicExperience, TriggerInitialMigration with profile validation
- **provider_connection_test.go**: ConnectProvider (dexcom, twiist, any), UpdatePatientDataSources, outbox events
- **redox_test.go**: ProcessEHRMessage, MatchClinicAndPatient (with/without action), SyncEHRData, SyncEHRDataForPatient, UpdatePatientSummary
- **xealth_test.go**: Full Xealth lifecycle (preorder, notification, programs, program URL, PDF report, cancel, pediatric flow)
- **serviceaccount_test.go**: AddServiceAccount, listing as service account
- **patientsites_test.go**: Patient site assignment/update (service-layer test)

## New Test Files

All new files go in `/integration/`. They reuse the existing test infrastructure: `setupEnvironment()`, `prepareRequest()`, `asClinician()`, `asServer()`, fixture loading, and the shared `server` (Echo HTTP handler).

---

### File 1: `integration/clinicians_test.go`

**Describe("Clinician Management", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic for clinician tests | POST `/v1/clinics` | POST | Setup |
| 2 | Create a second clinician (CLINIC_MEMBER role) | POST `/v1/clinics/{id}/clinicians` | POST | Success path |
| 3 | List clinicians for the clinic | GET `/v1/clinics/{id}/clinicians` | GET | Returns both admin + member |
| 4 | List clinicians filtered by role=CLINIC_MEMBER | GET `/v1/clinics/{id}/clinicians` | GET | Role filter |
| 5 | Get clinician by ID | GET `/v1/clinics/{id}/clinicians/{cid}` | GET | Success path |
| 6 | Update clinician role to CLINIC_ADMIN | PUT `/v1/clinics/{id}/clinicians/{cid}` | PUT | Role update, UpdatedBy tracked |
| 7 | List all clinicians across clinics | GET `/v1/clinicians` | GET | Cross-clinic listing |
| 8 | List clinics for a clinician | GET `/v1/clinicians/{userId}/clinics` | GET | Returns clinic details |
| 9 | Delete clinician captures deletedByUserId | DELETE `/v1/clinics/{id}/clinicians/{cid}` | DELETE | Auth user tracked in deletion metadata |

**Fixtures needed**: `integration/test/clinicians_fixtures/01_create_clinic.json`, `02_create_clinician.json`, `03_update_clinician.json`

---

### File 2: `integration/invites_test.go`

**Describe("Clinician Invites", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic for invite tests | POST `/v1/clinics` | POST | Setup |
| 2 | Create an invited clinician (no UserId, with InviteId) | POST `/v1/clinics/{id}/clinicians` | POST | Invite creation path |
| 3 | Get invited clinician by invite ID | GET `/v1/clinics/{id}/invites/clinicians/{inviteId}/clinician` | GET | Success path |
| 4 | Associate invited clinician to a user | PATCH `/v1/clinics/{id}/invites/clinicians/{inviteId}/clinician` | PATCH | Links invite to real user |
| 5 | Verify clinician now appears in clinician list | GET `/v1/clinics/{id}/clinicians` | GET | Confirm association |
| 6 | Create another invite and delete it | DELETE `/v1/clinics/{id}/invites/clinicians/{inviteId}/clinician` | DELETE | Invite deletion |
| 7 | Verify deleted invite returns 404 | GET `/v1/clinics/{id}/invites/clinicians/{inviteId}/clinician` | GET | Not found after delete |

**Fixtures needed**: `integration/test/invites_fixtures/01_create_clinic.json`, `02_create_invite.json`, `03_associate_invite.json`, `04_create_second_invite.json`

---

### File 3: `integration/patienttags_test.go`

**Describe("Patient Tags", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic | POST `/v1/clinics` | POST | Setup |
| 2 | Create patient tag | POST `/v1/clinics/{id}/patient_tags` | POST | Success, tag returned in clinic |
| 3 | Create second patient tag | POST `/v1/clinics/{id}/patient_tags` | POST | Multiple tags |
| 4 | Update patient tag name | PUT `/v1/clinics/{id}/patient_tags/{tagId}` | PUT | Success path |
| 5 | Verify updated tag via GetClinic | GET `/v1/clinics/{id}` | GET | Tag name changed |
| 6 | Create two patients | POST `/v1/clinics/{id}/patients` | POST | Setup for tag assignment |
| 7 | Assign tag to specific patients | POST `/v1/clinics/{id}/patients/assign_tag/{tagId}` | POST | With patient ID list |
| 8 | Verify patients have tag assigned | GET `/v1/clinics/{id}/patients/{pid}` | GET | Tag in patient.Tags |
| 9 | Assign tag to all patients (empty body) | POST `/v1/clinics/{id}/patients/assign_tag/{tagId}` | POST | Nil body = all patients |
| 10 | Delete tag from specific patients | POST `/v1/clinics/{id}/patients/delete_tag/{tagId}` | POST | With patient ID list |
| 11 | Delete tag from all patients (empty body) | POST `/v1/clinics/{id}/patients/delete_tag/{tagId}` | POST | Nil body = all patients |
| 12 | Delete patient tag | DELETE `/v1/clinics/{id}/patient_tags/{tagId}` | DELETE | Tag removed from clinic |
| 13 | Verify deleted tag gone from clinic | GET `/v1/clinics/{id}` | GET | Confirm removal |

**Fixtures needed**: `integration/test/patienttags_fixtures/01_create_clinic.json`, `02_create_tag.json`, `03_update_tag.json`, `04_create_patient_a.json`, `05_create_patient_b.json`

---

### File 4: `integration/sites_test.go`

**Describe("Sites", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic | POST `/v1/clinics` | POST | Setup |
| 2 | Create site A | POST `/v1/clinics/{id}/sites` | POST | Success path |
| 3 | Create site B | POST `/v1/clinics/{id}/sites` | POST | Second site |
| 4 | Verify sites in clinic via GetClinic | GET `/v1/clinics/{id}` | GET | Sites array populated |
| 5 | Update site A name | PUT `/v1/clinics/{id}/sites/{siteId}` | PUT | Name change |
| 6 | Create patient tag and convert to site | POST `/v1/clinics/{id}/patient_tags` then POST `/v1/clinics/{id}/patient_tags/{tagId}/site` | POST | ConvertPatientTagToSite |
| 7 | Merge site B into site A | POST `/v1/clinics/{id}/sites/{siteId}/merge` | POST | Merge two sites |
| 8 | Verify merged site gone, target updated | GET `/v1/clinics/{id}` | GET | One fewer site |
| 9 | Delete remaining site | DELETE `/v1/clinics/{id}/sites/{siteId}` | DELETE | Success path |

**Fixtures needed**: `integration/test/sites_fixtures/01_create_clinic.json`, `02_create_site.json`, `03_update_site.json`, `04_create_tag.json`, `05_merge_site.json`

---

### File 5: `integration/patients_test.go`

**Describe("Patient Management", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic | POST `/v1/clinics` | POST | Setup |
| 2 | Create custodial patient account | POST `/v1/clinics/{id}/patients` | POST | CreatePatientAccount: sets CustodialAccountPermissions, InvitedBy |
| 3 | Get patient by ID | GET `/v1/clinics/{id}/patients/{pid}` | GET | Success |
| 4 | Update patient (name, MRN, tags) | PUT `/v1/clinics/{id}/patients/{pid}` | PUT | Field update |
| 5 | List patients with search filter | GET `/v1/clinics/{id}/patients?search=...` | GET | Search filter |
| 6 | Create patient from existing user | POST `/v1/clinics/{id}/patients/{userId}` | POST | CreatePatientFromUser links user to clinic |
| 7 | Update patient permissions | PUT `/v1/clinics/{id}/patients/{pid}/permissions` | PUT | Permissions updated |
| 8 | Delete a specific permission | DELETE `/v1/clinics/{id}/patients/{pid}/permissions/{perm}` | DELETE | Single permission removed |
| 9 | List clinics for patient | GET `/v1/patients/{userId}/clinics` | GET | Patient sees their clinics |
| 10 | Get patient count | GET `/v1/clinics/{id}/patient_count` | GET | Returns count |
| 11 | Refresh patient count | POST `/v1/clinics/{id}/patient_count/refresh` | POST | Triggers recount |
| 12 | Send upload reminder | POST `/v1/clinics/{id}/patients/{pid}/upload_reminder` | POST | Must be user (not server), updates lastUploadReminderTime |
| 13 | Find patients cross-clinic | GET `/v1/patients?search=...` | GET | FindPatients requires user auth, queries across clinics |

**Fixtures needed**: `integration/test/patients_fixtures/01_create_clinic.json`, `02_create_patient.json`, `03_update_patient.json`, `04_create_patient_from_user.json`, `05_update_permissions.json`

---

### File 6: `integration/reviews_test.go`

**Describe("Patient Reviews", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic and patient | POST `/v1/clinics` + POST `/v1/clinics/{id}/patients` | POST | Setup |
| 2 | Add a review (as clinician) | PUT `/v1/clinics/{id}/patients/{pid}/reviews` | PUT | Requires user access (not server) |
| 3 | Verify review appears on patient | GET `/v1/clinics/{id}/patients/{pid}` | GET | Review with clinicianId + time |
| 4 | Delete review (as same clinician) | DELETE `/v1/clinics/{id}/patients/{pid}/reviews` | DELETE | Only review owner can delete |
| 5 | Verify review removed | GET `/v1/clinics/{id}/patients/{pid}` | GET | Reviews empty |

**Fixtures needed**: `integration/test/reviews_fixtures/01_create_clinic.json`, `02_create_patient.json`

---

### File 7: `integration/settings_test.go`

**Describe("Clinic Settings", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic | POST `/v1/clinics` | POST | Setup |
| 2 | Get EHR settings when none set (404) | GET `/v1/clinics/{id}/settings/ehr` | GET | Nil check → NotFound |
| 3 | Update EHR settings | PUT `/v1/clinics/{id}/settings/ehr` | PUT | Creates settings |
| 4 | Get EHR settings (200) | GET `/v1/clinics/{id}/settings/ehr` | GET | Returns settings |
| 5 | Get MRN settings when none set (404) | GET `/v1/clinics/{id}/settings/mrn` | GET | Nil check → NotFound |
| 6 | Update MRN settings | PUT `/v1/clinics/{id}/settings/mrn` | PUT | Creates settings |
| 7 | Get MRN settings (200) | GET `/v1/clinics/{id}/settings/mrn` | GET | Returns settings |
| 8 | Get patient count settings when none (404) | GET `/v1/clinics/{id}/settings/patient_count` | GET | Nil check → NotFound |
| 9 | Update patient count settings | PUT `/v1/clinics/{id}/settings/patient_count` | PUT | Creates settings + validates IsValid() |
| 10 | Get patient count settings (200) | GET `/v1/clinics/{id}/settings/patient_count` | GET | Returns settings |
| 11 | Update tier | POST `/v1/clinics/{id}/tier` | POST | Tier change |
| 12 | Update suppressed notifications | POST `/v1/clinics/{id}/suppressed_notifications` | POST | Notification suppression |
| 13 | Get clinic by share code | GET `/v1/clinics/share_code/{code}` | GET | Lookup via share code |
| 14 | Get clinic by invalid share code (404) | GET `/v1/clinics/share_code/nonexistent` | GET | NotFound |
| 15 | List membership restrictions | GET `/v1/clinics/{id}/membership_restrictions` | GET | Empty initially |
| 16 | Update membership restrictions | PUT `/v1/clinics/{id}/membership_restrictions` | PUT | Set restrictions |
| 17 | List membership restrictions (populated) | GET `/v1/clinics/{id}/membership_restrictions` | GET | Returns updated |

**Fixtures needed**: `integration/test/settings_fixtures/01_create_clinic.json`, `02_ehr_settings.json`, `03_mrn_settings.json`, `04_patient_count_settings.json`, `05_update_tier.json`, `06_suppressed_notifications.json`, `07_membership_restrictions.json`

---

### File 8: `integration/clinicusers_test.go`

**Describe("Clinic User Operations", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic with patient and clinician | POST `/v1/clinics` + patients + clinicians | POST | Setup |
| 2 | Update clinic user details (email) | POST `/v1/users/{userId}/clinics` | POST | Updates email across clinicians + patients |
| 3 | Verify email updated on patient record | GET `/v1/clinics/{id}/patients/{pid}` | GET | Email changed |
| 4 | Delete user from all clinics | DELETE `/v1/users/{userId}/clinics` | DELETE | Cascading delete of patient + clinician records |
| 5 | Verify patient removed | GET `/v1/clinics/{id}/patients/{pid}` | GET | 404 |
| 6 | Verify clinician removed | GET `/v1/clinics/{id}/clinicians/{cid}` | GET | 404 |

**Fixtures needed**: `integration/test/clinicusers_fixtures/01_create_clinic.json`, `02_create_patient.json`, `03_create_clinician.json`, `04_update_user_details.json`

---

### File 9: `integration/summaries_test.go`

**Describe("Patient Summaries", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Create clinic and patient | POST `/v1/clinics` + POST `/v1/clinics/{id}/patients` | POST | Setup |
| 2 | Update patient summary (with body) | POST `/v1/patients/{pid}/summary` | POST | Summary data provided |
| 3 | Update patient summary (empty body) | POST `/v1/patients/{pid}/summary` | POST | ContentLength == 0 path |
| 4 | View TIDE report | GET `/v1/clinics/{id}/tide_report` | GET | Returns report data |
| 5 | Delete patient summary | DELETE `/v1/summaries/{summaryId}/clinics` | DELETE | Removes from all clinics |

**Fixtures needed**: `integration/test/summaries_fixtures/01_create_clinic.json`, `02_create_patient.json`, `03_update_summary.json`

---

### File 10: `integration/redox_verify_test.go`

**Describe("Redox Verification", Ordered)**

| # | Scenario | Endpoint | Method | Key Branch |
|---|----------|----------|--------|------------|
| 1 | Verify Redox endpoint (challenge response) | POST `/v1/redox/verify` | POST | Returns challenge back |

**Fixtures needed**: `integration/test/redox_fixtures/06_verify_request.json`

---

## Implementation Approach

### Test Infrastructure
- Reuse the existing `setupEnvironment()` in `integration_suite_test.go` (Shoreline, Seagull, Auth, Keycloak stubs)
- Reuse `prepareRequest()`, `asClinician()`, `asServer()`, `asServiceAccount()` helpers
- All new tests use `Ordered` Ginkgo blocks following existing convention
- JSON fixtures for request payloads in `integration/test/{feature}_fixtures/`
- Assertions use `Expect(rec.Result().StatusCode).To(Equal(...))` and JSON unmarshaling

### Stubbing Strategy
- **No new stubs needed** — existing HTTP stubs (Shoreline, Seagull, Auth, Keycloak, Xealth) cover all external dependencies
- For `CreatePatientFromUser`, the Shoreline stub already handles `GetUser` and Seagull stub handles profile lookup
- For `FindPatients`, the clinician lookup uses the real DB (populated during test setup)

### Key Conventions from Existing Tests
- Each `Describe` block is `Ordered` to allow sequential dependent steps
- Clinic IDs, patient IDs, clinician IDs stored in package-level `var` for use across steps
- `rec := httptest.NewRecorder()` + `server.ServeHTTP(rec, req)` pattern
- Fixture files are plain JSON loaded via `prepareRequest(method, path, fixturePath)`
- Response bodies unmarshaled to verify specific fields

## Verification

1. Run existing integration tests to confirm they still pass: `cd integration && go test -v ./...`
2. Run new tests: same command — all tests in `/integration/` run together sharing the same suite setup
3. Verify no test interdependencies by running the full suite multiple times
4. Check MongoDB cleanup between test suites (handled by `TeardownDatabase`)

## Files to Create/Modify

**New files:**
- `integration/clinicians_test.go`
- `integration/invites_test.go`
- `integration/patienttags_test.go`
- `integration/sites_test.go`
- `integration/patients_test.go`
- `integration/reviews_test.go`
- `integration/settings_test.go`
- `integration/clinicusers_test.go`
- `integration/summaries_test.go`
- `integration/redox_verify_test.go`
- ~25 fixture JSON files in `integration/test/*_fixtures/` directories

**Existing files (reference only, not modified):**
- `integration/integration_suite_test.go` — test setup and helpers
- `integration/test/stubs.go` — HTTP stubs for external services
- `api/clinicians.go`, `api/clinics.go`, `api/patients.go`, `api/redox.go`, `api/xealth.go` — handlers under test
