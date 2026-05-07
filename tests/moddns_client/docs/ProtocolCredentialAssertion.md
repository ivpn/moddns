# ProtocolCredentialAssertion


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**mediation** | [**ProtocolCredentialMediationRequirement**](ProtocolCredentialMediationRequirement.md) |  | [optional] 
**public_key** | [**ProtocolPublicKeyCredentialRequestOptions**](ProtocolPublicKeyCredentialRequestOptions.md) |  | [optional] 

## Example

```python
from moddns.models.protocol_credential_assertion import ProtocolCredentialAssertion

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolCredentialAssertion from a JSON string
protocol_credential_assertion_instance = ProtocolCredentialAssertion.from_json(json)
# print the JSON string representation of the object
print(ProtocolCredentialAssertion.to_json())

# convert the object into a dict
protocol_credential_assertion_dict = protocol_credential_assertion_instance.to_dict()
# create an instance of ProtocolCredentialAssertion from a dict
protocol_credential_assertion_from_dict = ProtocolCredentialAssertion.from_dict(protocol_credential_assertion_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


