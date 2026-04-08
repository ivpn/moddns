# ResponsesWebAuthnReauthFinishResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**expires_at** | **str** |  | [optional] 
**reauth_token** | **str** |  | [optional] 

## Example

```python
from moddns.models.responses_web_authn_reauth_finish_response import ResponsesWebAuthnReauthFinishResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ResponsesWebAuthnReauthFinishResponse from a JSON string
responses_web_authn_reauth_finish_response_instance = ResponsesWebAuthnReauthFinishResponse.from_json(json)
# print the JSON string representation of the object
print(ResponsesWebAuthnReauthFinishResponse.to_json())

# convert the object into a dict
responses_web_authn_reauth_finish_response_dict = responses_web_authn_reauth_finish_response_instance.to_dict()
# create an instance of ResponsesWebAuthnReauthFinishResponse from a dict
responses_web_authn_reauth_finish_response_from_dict = ResponsesWebAuthnReauthFinishResponse.from_dict(responses_web_authn_reauth_finish_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


