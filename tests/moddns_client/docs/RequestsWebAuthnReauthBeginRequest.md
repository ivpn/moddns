# RequestsWebAuthnReauthBeginRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**purpose** | **str** |  | 

## Example

```python
from moddns.models.requests_web_authn_reauth_begin_request import RequestsWebAuthnReauthBeginRequest

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsWebAuthnReauthBeginRequest from a JSON string
requests_web_authn_reauth_begin_request_instance = RequestsWebAuthnReauthBeginRequest.from_json(json)
# print the JSON string representation of the object
print(RequestsWebAuthnReauthBeginRequest.to_json())

# convert the object into a dict
requests_web_authn_reauth_begin_request_dict = requests_web_authn_reauth_begin_request_instance.to_dict()
# create an instance of RequestsWebAuthnReauthBeginRequest from a dict
requests_web_authn_reauth_begin_request_from_dict = RequestsWebAuthnReauthBeginRequest.from_dict(requests_web_authn_reauth_begin_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


