# ApiWebAuthnLoginBeginRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**email** | **str** |  | 

## Example

```python
from moddns.models.api_web_authn_login_begin_request import ApiWebAuthnLoginBeginRequest

# TODO update the JSON string below
json = "{}"
# create an instance of ApiWebAuthnLoginBeginRequest from a JSON string
api_web_authn_login_begin_request_instance = ApiWebAuthnLoginBeginRequest.from_json(json)
# print the JSON string representation of the object
print(ApiWebAuthnLoginBeginRequest.to_json())

# convert the object into a dict
api_web_authn_login_begin_request_dict = api_web_authn_login_begin_request_instance.to_dict()
# create an instance of ApiWebAuthnLoginBeginRequest from a dict
api_web_authn_login_begin_request_from_dict = ApiWebAuthnLoginBeginRequest.from_dict(api_web_authn_login_begin_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


