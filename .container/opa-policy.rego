package envoy.authz

import input.attributes.request.http as http_request

default allow := false

allow if {
   http_request.method == "GET"
}
