# ProtocolCredentialDescriptor


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **List[int]** | CredentialID The ID of a credential to allow/disallow. | [optional] 
**transports** | [**List[ProtocolAuthenticatorTransport]**](ProtocolAuthenticatorTransport.md) | The authenticator transports that can be used. | [optional] 
**type** | [**ProtocolCredentialType**](ProtocolCredentialType.md) | The valid credential types. | [optional] 

## Example

```python
from moddns.models.protocol_credential_descriptor import ProtocolCredentialDescriptor

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolCredentialDescriptor from a JSON string
protocol_credential_descriptor_instance = ProtocolCredentialDescriptor.from_json(json)
# print the JSON string representation of the object
print(ProtocolCredentialDescriptor.to_json())

# convert the object into a dict
protocol_credential_descriptor_dict = protocol_credential_descriptor_instance.to_dict()
# create an instance of ProtocolCredentialDescriptor from a dict
protocol_credential_descriptor_from_dict = ProtocolCredentialDescriptor.from_dict(protocol_credential_descriptor_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


