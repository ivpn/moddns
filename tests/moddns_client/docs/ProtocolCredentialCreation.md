# ProtocolCredentialCreation


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**mediation** | [**ProtocolCredentialMediationRequirement**](ProtocolCredentialMediationRequirement.md) |  | [optional] 
**public_key** | [**ProtocolPublicKeyCredentialCreationOptions**](ProtocolPublicKeyCredentialCreationOptions.md) |  | [optional] 

## Example

```python
from moddns.models.protocol_credential_creation import ProtocolCredentialCreation

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolCredentialCreation from a JSON string
protocol_credential_creation_instance = ProtocolCredentialCreation.from_json(json)
# print the JSON string representation of the object
print(ProtocolCredentialCreation.to_json())

# convert the object into a dict
protocol_credential_creation_dict = protocol_credential_creation_instance.to_dict()
# create an instance of ProtocolCredentialCreation from a dict
protocol_credential_creation_from_dict = ProtocolCredentialCreation.from_dict(protocol_credential_creation_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


