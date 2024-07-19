package http.authz.clinic

subject_id := input.auth.subjectId
is_backend_service := input.auth.serverAccess == true

is_authenticated_user {
  not is_backend_service
  subject_id
}

read_access_roles := {
  "CLINIC_MEMBER",
  "CLINIC_ADMIN"
}

write_access_roles := {
  "CLINIC_ADMIN"
}

# convert clinician roles to set
clinician_roles := { x | x = input.clinician.roles[_] }

clinician_has_read_access {
  count(clinician_roles & read_access_roles) > 0
}

clinician_has_write_access {
  count(clinician_roles & write_access_roles) > 0
}

default allow = false

# Allow backend services to list all clinics
# GET /v1/clinics
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics"]
}

# Allow authenticated users to create a new clinic
# POST /v1/clinics
allow {
  is_authenticated_user
  input.method == "POST"
  input.path = ["v1", "clinics"]
}

# Allow currently authenticated user to fetch the clinics they are a patient of
# GET /v1/patients/:patientId/clinics when ":patientId" == auth_subject_id
allow {
  is_authenticated_user
  some patient_id
  input.method == "GET"
  input.path = ["v1", "patients", patient_id, "clinics"]
  patient_id == subject_id
}

# Allow backend services to fetch list of clinics a patient belongs to
# GET /v1/patients/:patientId/clinics
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "patients", _, "clinics"]
}

# Allow backend services update patient summaries
# GET /v1/patients/:patientId/clinics
allow {
  is_backend_service
  input.method == "POST"
  input.path = ["v1", "patients", _, "summary"]
}

# Allow backend services to remove custodial permission
# DELETE /v1/clinics/:clinicId/patients/:patientId/permissions/custodian
allow {
  is_backend_service
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "patients", _, "permissions", "custodian"]
}

# Allow currently authenticated user to delete permissions they have granted
# DELETE /v1/clinics/:clinicId/patients/:patientId/permissions/custodian when ":patientId" == auth_subject_id
allow {
  is_authenticated_user
  some patient_id
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "patients", patient_id, "permissions", _]
  patient_id == subject_id
}

# Allow currently authenticated user to fetch the clinics they are a member of
# GET /v1/clinicians/:clinicianId/clinics when ":clinicianId" == auth_subject_id
allow {
  is_authenticated_user
  some clinician_id
  input.method == "GET"
  input.path = ["v1", "clinicians", clinician_id, "clinics"]
  clinician_id == subject_id
}

# Allow backend services to fetch the clinics membership information for a given user
# GET /v1/clinicians/:clinicianId/clinics
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinicians", clinician_id, "clinics"]
}

# Allow currently authenticated user to change the permissions they have granted to a clinic
# PUT /v1/clinics/:clinicId/patients/:patientId/permissions when ":patientId" == auth_subject_id
allow {
  is_authenticated_user
  some patient_id
  input.method == "PUT"
  input.path = ["v1", "clinics", _, "patients", patient_id, "permissions"]
  patient_id == subject_id
}

# Allow authenticated users to fetch clinics by id
# GET /v1/clinics/:clinicId
allow {
  is_authenticated_user
  input.method == "GET"
  input.path = ["v1", "clinics", _]
}

# Allow backend services to fetch clinics by id
# GET /v1/clinics/:clinicId
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _]
}

# Allow authenticated users to fetch clinics by share code
# GET /v1/clinics/share_code/:shareCode
allow {
  is_authenticated_user
  input.method == "GET"
  input.path = ["v1", "clinics", "share_code", _]
}

# Allow backend services to fetch a clinic by id
# GET /v1/clinics/:clinicId
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _]
}

# Allow currently authenticated clinician to update clinic
# PUT /v1/clinics/:clinicId
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _]
  clinician_has_write_access
}

# Allow currently authenticated clinician to delete clinic
# PUT /v1/clinics/:clinicId
allow {
  input.method == "DELETE"
  input.path = ["v1", "clinics", _]
  clinician_has_write_access
}

# Allow backend services to update clinic tiers
# POST /v1/clinics/:clinicId/tier
allow {
  is_backend_service
  input.method == "POST"
  input.path = ["v1", "clinics", _, "tier"]
}

# Allow backend services to update clinic tiers
# POST /v1/clinics/:clinicId/suppressed_notifications
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "suppressed_notifications"]
  clinician_has_write_access
}

# Allow backend services to add service accounts to clinics
# POST /v1/clinics/:clinicId/service_accounts
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "service_accounts"]
  is_backend_service
}

# Allow currently authenticated clinician to list clinicians
# GET /v1/clinics/:clinicId/clinicians
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians"]
  clinician_has_read_access
}

# Allow backend services to list clinicians
# GET /v1/clinics/:clinicId/clinicians
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians"]
}

# Allow backend services to create clinicians
# POST /v1/clinics/:clinicId/clinicians
allow {
  is_backend_service
  input.method == "POST"
  input.path = ["v1", "clinics", _, "clinicians"]
}

# Allow currently authenticated clinician to get a clinician by id
# GET /v1/clinics/:clinicId/clinicians/:clinicianId
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians", _]
  clinician_has_read_access
}

# Allow backend services to fetch clinician records
# GET /v1/clinics/:clinicId/clinicians/:clinicianId
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians", _]
}

# Allow backend services to fetch clinicians list
# GET /v1/clinics/:clinicId/clinicians
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians"]
}

# Allow currently authenticated clinician to update or delete a clinician
# PUT /v1/clinics/:clinicId/clinicians/:clinicianId
# DELETE /v1/clinics/:clinicId/clinicians/:clinicianId
allow {
  allowed_methods := {"PUT", "DELETE"}
  allowed_methods[input.method]
  input.path = ["v1", "clinics", _, "clinicians", _]
  clinician_has_write_access
}

# Allow currently authenticated clinician to remove themselves from the clinic
# DELETE /v1/clinics/:clinicId/clinicians/:clinicianId
allow {
  is_authenticated_user
  some clinician_id
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "clinicians", clinician_id]
  clinician_id == subject_id
  clinician_has_read_access
}

# Allow currently authenticated patient to remove themselves from the clinic
# DELETE /v1/clinics/:clinicId/patients/:patientId
allow {
  is_authenticated_user
  some patient_id
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "patients", patient_id]
  patient_id == subject_id
}

# Allow currently authenticated clinic admin to remove patients from a clinic
# DELETE /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "patients", _]
  clinician_has_write_access
}

# Allow backend services to fetch, update and delete invites
# GET /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
# PATCH /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
# DELETE /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
allow {
  is_backend_service
  allowed_methods := {"GET", "PATCH", "DELETE"}
  allowed_methods[input.method]
  input.path = ["v1", "clinics", _, "invites", "clinicians", _, "clinician"]
}

# Allow currently authenticated clinician to list patients
# GET /v1/clinics/:clinicId/patients
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patients"]
  clinician_has_read_access
}

# Allow backend services to list patients
# GET /v1/clinics/:clinicId/patients
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patients"]
}

# Allow currently authenticated clinician to get tide reports
# GET /v1/clinics/:clinicId/tide_report
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "tide_report"]
  clinician_has_read_access
}

# Allow backend services to get tide reports
# GET /v1/clinics/:clinicId/tide_report
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _, "tide_report"]
}

# Allow currently authenticated clinician to create a custodial account
# POST /v1/clinics/:clinicId/patients
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients"]
  clinician_has_read_access
}

# Allow backend service create a custodial accounts
# POST /v1/clinics/:clinicId/patients
allow {
  is_backend_service
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients"]
}

# Allow currently authenticated clinician to send an upload reminder
# POST /v1/clinics/:clinicId/patients
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients", _, "upload_reminder"]
  clinician_has_read_access
}

# Allow currently authenticated clinician to send a dexcom connect reminder
# POST /v1/clinics/:clinicId/patients/:patientId/send_dexcom_connect_request
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients", _, "send_dexcom_connect_request"]
  clinician_has_read_access
}

# Allow currently authenticated clinician to fetch patient by id
# GET /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patients", _]
  clinician_has_read_access
}

# Allow backend services to fetch patient by id
# GET /v1/clinics/:clinicId/patients/:patientId
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patients", _]
}

# Allow backend services to create a patient from existing user
# POST /v1/clinics/:clinicId/patients/:patientId
allow {
  is_backend_service
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients", _]
}

# Allow currently authenticated clinician to update patient account
# PUT /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _, "patients", _]
  clinician_has_read_access
}

# Allow backend services to create an empty clinic for legacy clinician
# POST /v1/clinicians/:userId/migrate
allow {
  is_backend_service
  input.method == "POST"
  input.path = ["v1", "clinicians", _, "migrate"]
}

# Allow currently authenticated clinician to trigger the initial migration
# POST /v1/clinics/:clinicId/migrate
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "migrate"]
  clinician_has_write_access
}

# Allow backend services to migrate users of a legacy clinician account to the clinic
# POST /v1/clinics/:clinicId/migrations
allow {
  is_backend_service
  input.method == "POST"
  input.path = ["v1", "clinics", _, "migrations"]
}

# Allow backend services to access the list of migrations for a given clinic
# GET /v1/clinics/:clinicId/migrations
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _, "migrations"]
}

# Allow backend services to fetch migrations by id
# GET /v1/clinics/:clinicId/migrations/:userId
allow {
  is_backend_service
  input.method == "GET"
  input.path = ["v1", "clinics", _, "migrations", _]
}

# Allow clinicians to access all migrations
# GET /v1/clinics/:clinicId/migrations/:userId
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "migrations", _]
  clinician_has_read_access
}

# Allow clinicians to list all migrations
# GET /v1/clinics/:clinicId/migrations
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "migrations"]
  clinician_has_read_access
}

# Allow backend services to update the status of migrations
# PATCH /v1/clinics/:clinicId/migrations/:userId
allow {
  is_backend_service
  input.method == "PATCH"
  input.path = ["v1", "clinics", _, "migrations", _]
}

# Allow backend services to update the status of migrations
# PATCH /v1/users/:clinicId/clinics
allow {
  is_backend_service
  input.method == "DELETE"
  input.path = ["v1", "users", _, "clinics"]
}
# Allow backend services to update the status of migrations
# PATCH /v1/users/:clinicId/clinics
allow {
  is_backend_service
  input.method == "DELETE"
  input.path = ["v1", "users", _, "clinics"]
}

# Allow backend services to update user details
# POST /v1/users/:clinicId/clinics
allow {
  is_backend_service
  input.method == "POST"
  input.path = ["v1", "users", _, "clinics"]
}

# Allow currently authenticated clinic member to create a patient tag
# POST /v1/clinics/:clinicId/patient_tags
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patient_tags"]
  clinician_has_read_access
}

# Allow currently authenticated clinic member to update a patient tag
# PUT /v1/clinics/:clinicId/patient_tags/:patientTagId
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _, "patient_tags", _]
  clinician_has_read_access
}

# Allow currently authenticated clinic admin to delete a patient tag
# DELETE /v1/clinics/:clinicId/patient_tags/:patientTagId
allow {
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "patient_tags", _]
  clinician_has_write_access
}

# Allow backend services or clinic admins to delete a patient tag from all clinic patients
# POST /v1/clinics/:clinicId/patients/delete_tag/:patientTagId
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients", "delete_tag", _]
  is_backend_service
}
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients", "delete_tag", _]
  clinician_has_write_access
}

# Allow backend services or clinic admins to assign a patient tag to a subset of clinic patients
# POST /v1/clinics/:clinicId/patients/assign_tag/:patientTagId
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients", "assign_tag", _]
  is_backend_service
}
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients", "assign_tag", _]
  clinician_has_write_access
}

# Allow backend services to update a user data source for all associated clinic patient records
# PUT /v1/patients/:patientId/data_sources
allow {
  input.method == "PUT"
  input.path = ["v1", "patients", _, "data_sources"]
  is_backend_service
}

# Allow backend services to update clinic membership restrictions
# PUT /v1/clinics/:clinicId/membership_restrictions
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _, "membership_restrictions"]
  is_backend_service
}

# Allow backend services to list clinic membership restrictions
# GET /v1/clinics/:clinicId/membership_restrictions
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "membership_restrictions"]
  is_backend_service
}

# Allow clinic admins to list clinic membership restrictions
# GET /v1/clinics/:clinicId/membership_restrictions
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "membership_restrictions"]
  clinician_has_write_access
}

# Allow services to fetch clinics settings
# GET /v1/clinics/:clinicId/settings/:settings
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "settings", _]
  is_backend_service
}

# Allow clinic members to fetch mrn settings
# GET /v1/clinics/:clinicId/settings/mrn
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "settings", "mrn"]
  clinician_has_read_access
}

# Allow clinic members to fetch ehr settings
# GET /v1/clinics/:clinicId/settings/ehr
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "settings", "ehr"]
  clinician_has_read_access
}

# Allow clinic members to fetch patient count settings
# GET /v1/clinics/:clinicId/settings/patient_count
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "settings", "patient_count"]
  clinician_has_read_access
}

# Allow services to update clinics settings
# GET /v1/clinics/:clinicId/settings/:settings
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _, "settings", _]
  is_backend_service
}

# Allow services to update clinics settings
# GET /v1/clinics/:clinicId/settings/:settings
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _, "settings", _]
  is_backend_service
}

# Allow clinic members to fetch patient count
# GET /v1/clinics/:clinicId/patient_count
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patient_count"]
  clinician_has_read_access
}

# Allow services to fetch patient count
# GET /v1/clinics/:clinicId/patient_count
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patient_count"]
  is_backend_service
}

# Allow services to match EHR patients
# GET /v1/redox/match
allow {
  input.method == "POST"
  input.path = ["v1", "redox", "match"]
  is_backend_service
}

# Allow services to trigger EHR data sync for an entire clinic
# GET /v1/clinics/:clinicId/ehr/sync
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "ehr", "sync"]
  is_backend_service
}

# Allow services to trigger EHR data sync for a patient
# GET /v1/patients/:patientId/ehr/sync
allow {
  input.method == "POST"
  input.path = ["v1", "patients", _, "ehr", "sync"]
  is_backend_service
}

# Allow any authenticated user to fetch patients they have access to
# GET /v1/patients
allow {
  input.method == "GET"
  input.path = ["v1", "patients"]
  is_authenticated_user
}

# Allow currently authenticated clinician to set reviewed
# PUT /v1/clinics/:clinicId/patients/:patientId/reviews
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _, "patients", _, "reviews"]
  clinician_has_read_access
}

# Allow currently authenticated clinician to revert review
# DELETE /v1/clinics/:clinicId/patients/:patientId/reviews
allow {
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "patients", _, "reviews"]
  clinician_has_read_access
}