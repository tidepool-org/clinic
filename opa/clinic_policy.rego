package clinics

default allow = false

clinicService = "clinic"
clinicServicePort = "8080"


allow {
  any([input.method == "GET"])
  input.parsed_path = ["clinics"]

  # Get roles
  input_roles := {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN","CLINIC_CLINICIAN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "POST"])
  input.parsed_path = ["clinics"]

}

allow {
  any([input.method == "GET"])
  input.parsed_path = ["clinics",clinicid]

  # Get roles
  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])
  response := http.send({
    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},
    "method" : "GET",
    "url": url
  })

  # Get input roles from response
  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN","CLINIC_CLINICIAN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "PATCH",input.method == "DELETE"])
  input.parsed_path = ["clinics",clinicid]

  # Get roles
  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])
  response := http.send({
    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},
    "method" : "GET",
    "url": url
  })

  # Get input roles from response
  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "DELETE",input.method == "PATCH"])
  input.parsed_path = ["clinics",clinicid,"clinicians",clinicianid]

  # Get roles
  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])
  response := http.send({
    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},
    "method" : "GET",
    "url": url
  })

  # Get input roles from response
  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "GET"])
  input.parsed_path = ["clinics",clinicid,"clinicians",clinicianid]

  # Get roles
  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])
  response := http.send({
    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},
    "method" : "GET",
    "url": url
  })

  # Get input roles from response
  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN","CLINIC_CLINICIAN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "GET",input.method == "POST"])
  input.parsed_path = ["clinics",clinicid,"patients"]

  # Get roles
  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])
  response := http.send({
    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},
    "method" : "GET",
    "url": url
  })

  # Get input roles from response
  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN","CLINIC_CLINICIAN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "DELETE",input.method == "GET",input.method == "PATCH"])
  input.parsed_path = ["clinics",clinicid,"patients",patientid]

  # Get roles
  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])
  response := http.send({
    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},
    "method" : "GET",
    "url": url
  })

  # Get input roles from response
  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN","CLINIC_CLINICIAN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "GET"])
  input.parsed_path = ["clinics",clinicid,"clinicians"]

  # Get roles
  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])
  response := http.send({
    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},
    "method" : "GET",
    "url": url
  })

  # Get input roles from response
  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN","CLINIC_CLINICIAN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "POST"])
  input.parsed_path = ["clinics",clinicid,"clinicians"]

  # Get roles
  url := sprintf("http://%s:%s/clinics/%s/clinicians/%s", [clinicService, clinicServicePort, clinicid, input.user_id])
  response := http.send({
    "headers": {"X-TIDEPOOL-USERID": "ADMIN"},
    "method" : "GET",
    "url": url
  })

  # Get input roles from response
  input_roles := {y | y = response.body.permissions[_]} | {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN","CLINIC_ADMIN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "GET",input.method == "DELETE"])
  input.parsed_path = ["clinics","patients",patientid]

  # Get roles
  input_roles := {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "GET",input.method == "DELETE"])
  input.parsed_path = ["clinics","clinicians",clinicianid]

  # Get roles
  input_roles := {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

allow {
  any([input.method == "GET"])
  input.parsed_path = ["clinics","access"]

  # Get roles
  input_roles := {x | x = input.roles[_]}
  roles := {"TIDEPOOL_ADMIN"}
  s := roles & input_roles

  # Make sure valid role exists
  count(s) > 0
}

