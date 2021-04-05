package http.authz.clinic

subject_id := input.headers["x-auth-subject-id"]
is_backend_service := input.headers["x-auth-server-access"] == "true"

is_authenticated_user {
  not backend_service
  subject_id
}

is_backend_service_any_of(services) {
  backend_service
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

# Allow currently authenticated user to fetch the clinics they are a member of
# GET /v1/patients/:patientId/clinics when ":patientId" == auth_subject_id
allow {
  is_authenticated_user
  input.method = "GET"
  input.path = ["v1", "patients", auth_subject_id, "clinics"]
}

# Allow currently authenticated user to change the permissions they have granted to a clinic
# PUT /v1/clinics/:clinicId/patients/:patientId/permissions when ":patientId" == auth_subject_id
allow {
  is_authenticated_user
  input.method = "PUT"
  input.path = ["v1", "clinics", _, "patients", auth_subject_id, "permissions"]
}

# Allow currently authenticated clinician to fetch clinic
# GET /v1/clinics/:clinicId
allow {
  input.method = "GET"
  input.path = ["v1", "clinics", _]
  clinician_has_read_access
}

# Allow currently authenticated clinician to update clinic
# PUT /v1/clinics/:clinicId
allow {
  input.method = "PUT"
  input.path = ["v1", "clinics", _]
  clinician_has_write_access
}

# Allow currently authenticated clinician to list clinicians
# GET /v1/clinics/:clinicId/clinicians
allow {
  input.method = "GET"
  input.path = ["v1", "clinics", _, "clinicians"]
  clinician_has_read_access
}

# Allow hydrophone to create clinicians
# POST /v1/clinics/:clinicId/clinicians
allow {
  input.method = "POST"
  input.path = ["v1", "clinics", _, "clinicians"]
  is_backend_service_any_of({"hydrophone"})
}

# Allow currently authenticated clinician to get a clinician by id
# GET /v1/clinics/:clinicId/clinicians/:clinicianId
allow {
  input.method = "GET"
  input.path = ["v1", "clinics", _, "clinicians", _]
  clinician_has_read_access
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

# Allow hydrophone to fetch, update and delete invites
# GET /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
# PUT /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
# DELETE /v1/clinics/:clinicId/invites/clinicians/:inviteId/clinician
allow {
  allowed_methods := {"GET", "PUT", "DELETE"}
  allowed_methods[input.method]
  input.path = ["v1", "clinics", _, "invites", "clinicians", _, "clinician"]
  is_backend_service_any_of({"hydrophone"})
}

# Allow currently authenticated clinician to list patients
# GET /v1/clinics/:clinicId/patients
allow {
  input.method = "GET"
  input.path = ["v1", "clinics", _, "patients"]
  clinician_has_read_access
}

# Allow currently authenticated clinician to create a custodial account
# GET /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method = "POST"
  input.path = ["v1", "clinics", _, "patients"]
  clinician_has_write_access
}

# Allow currently authenticated clinician to fetch patient by id
# GET /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method = "GET"
  input.path = ["v1", "clinics", _, "patients", _]
  clinician_has_read_access
}

# Allow hydrophone and prescription services to create patient from existing user
# POST /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method = "POST"
  input.path = ["v1", "clinics", _, "patients", _]
  is_backend_service_any_of({"hydrophone", "prescription"})
}

# Allow currently authenticated clinician to update patient account
# PUT /v1/clinics/:clinicId/patients/:patientId
allow {
  input.method = "PUT"
  input.path = ["v1", "clinics", _, "patients", _]
  clinician_has_write_access
}
