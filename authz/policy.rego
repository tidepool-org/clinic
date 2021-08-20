package http.authz.clinic

subject_id := input.headers["x-auth-subject-id"]
is_backend_service := input.headers["x-auth-server-access"] == "true"

is_authenticated_user {
  not is_backend_service
  subject_id
}

is_backend_service_any_of(services) {
  is_backend_service
  services[subject_id]
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

# Only allow backend services to list all clinics
# GET /v1/clinics
allow {
  is_backend_service_any_of({"orca", "hydrophone"})
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

# Allow shoreline to fetch list of clinics a patient belongs to
# GET /v1/patients/:patientId/clinics
allow {
  is_backend_service_any_of({"shoreline", "orca"})
  input.method == "GET"
  input.path = ["v1", "patients", _, "clinics"]
}

# Allow shoreline to remove custodial permission
# DELETE /v1/clinics/:clinicId/patients/:patientId/permissions/custodian
allow {
  is_backend_service_any_of({"shoreline"})
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "patients", _, "permissions", "custodian"]
}

# Allow currently authenticated user to delete permissions they have granted
# DELETE /v1/clinics/:clinicId/patients/:patientId/permissions/custodian when ":patientId" == auth_subject_id
allow {
  is_authenticated_user
  some patient_id
  input.method == "DELETE"
  input.path = ["v1", "clinics", _, "patients", patient_id, "permissions", "upload"]
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

# Allow authenticated users to fetch clinics by share code
# GET /v1/clinics/share_code/:shareCode
allow {
  is_authenticated_user
  input.method == "GET"
  input.path = ["v1", "clinics", "share_code", _]
}

# Allow hydrophone to fetch a clinic by id
# GET /v1/clinics/:clinicId
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _]
  is_backend_service_any_of({"hydrophone"})
}

# Allow currently authenticated clinician to update clinic
# PUT /v1/clinics/:clinicId
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _]
  clinician_has_write_access
}

# Allow currently authenticated clinician to list clinicians
# GET /v1/clinics/:clinicId/clinicians
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians"]
  clinician_has_read_access
}

# Allow hydrophone to create clinicians
# POST /v1/clinics/:clinicId/clinicians
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "clinicians"]
  is_backend_service_any_of({"hydrophone"})
}

# Allow currently authenticated clinician to get a clinician by id
# GET /v1/clinics/:clinicId/clinicians/:clinicianId
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians", _]
  clinician_has_read_access
}

# Allow hydrophone to fetch clinician records
# GET /v1/clinics/:clinicId/clinicians/:clinicianId
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians", _]
  is_backend_service_any_of({"hydrophone"})
}

# Allow hydrophone to fetch clinicians list
# GET /v1/clinics/:clinicId/clinicians
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "clinicians"]
  is_backend_service_any_of({"hydrophone"})
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

# Allow hydrophone to fetch, update and delete invites
# GET /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
# PATCH /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
# DELETE /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
allow {
  allowed_methods := {"GET", "PATCH", "DELETE"}
  allowed_methods[input.method]
  input.path = ["v1", "clinics", _, "invites", "clinicians", _, "clinician"]
  is_backend_service_any_of({"hydrophone"})
}

# Allow currently authenticated clinician to list patients
# GET /v1/clinics/:clinicId/patients
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patients"]
  clinician_has_read_access
}

# Allow currently authenticated clinician to create a custodial account
# POST /v1/clinics/:clinicId/patients
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients"]
  clinician_has_write_access
}

# Allow currently authenticated clinician to fetch patient by id
# GET /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patients", _]
  clinician_has_read_access
}

# Allow hydrophone to fetch patient by id
# GET /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "patients", _]
  is_backend_service_any_of({"hydrophone"})
}

# Allow clinic-worker, hydrophone, prescription services to create a patient from existing user
# POST /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "patients", _]
  is_backend_service_any_of({"hydrophone", "prescription", "clinic-worker"})
}

# Allow currently authenticated clinician to update patient account
# PUT /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method == "PUT"
  input.path = ["v1", "clinics", _, "patients", _]
  clinician_has_write_access
}

# Allow Orca to create an empty clinic for legacy clinician
# POST /v1/clinicians/:userId/migrate
allow {
  input.method == "POST"
  input.path = ["v1", "clinicians", _, "migrate"]
  is_backend_service_any_of({"orca"})
}

# Allow currently authenticated clinician to trigger the initial migration
# POST /v1/clinics/:clinicId/migrate
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "migrate"]
  clinician_has_write_access
}

# Allow Orca to migrate users of a legacy clinician account to the clinic
# POST /v1/clinics/:clinicId/migrations
allow {
  input.method == "POST"
  input.path = ["v1", "clinics", _, "migrations"]
  is_backend_service_any_of({"orca"})
}

# Allow Orca to access the list of migrations for a given clinic
# GET /v1/clinics/:clinicId/migrations
allow {
  input.method == "GET"
  input.path = ["v1", "clinics", _, "migrations"]
  is_backend_service_any_of({"orca"})
}
