# ProtocolPublicKeyCredentialRequestOptions


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**allow_credentials** | [**List[ProtocolCredentialDescriptor]**](ProtocolCredentialDescriptor.md) |  | [optional] 
**challenge** | **List[int]** |  | [optional] 
**extensions** | **Dict[str, object]** |  | [optional] 
**hints** | [**List[ProtocolPublicKeyCredentialHints]**](ProtocolPublicKeyCredentialHints.md) |  | [optional] 
**rp_id** | **str** |  | [optional] 
**timeout** | **int** |  | [optional] 
**user_verification** | [**ProtocolUserVerificationRequirement**](ProtocolUserVerificationRequirement.md) |  | [optional] 

## Example

```python
from moddns.models.protocol_public_key_credential_request_options import ProtocolPublicKeyCredentialRequestOptions

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolPublicKeyCredentialRequestOptions from a JSON string
protocol_public_key_credential_request_options_instance = ProtocolPublicKeyCredentialRequestOptions.from_json(json)
# print the JSON string representation of the object
print(ProtocolPublicKeyCredentialRequestOptions.to_json())

# convert the object into a dict
protocol_public_key_credential_request_options_dict = protocol_public_key_credential_request_options_instance.to_dict()
# create an instance of ProtocolPublicKeyCredentialRequestOptions from a dict
protocol_public_key_credential_request_options_from_dict = ProtocolPublicKeyCredentialRequestOptions.from_dict(protocol_public_key_credential_request_options_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


