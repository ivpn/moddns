# ProtocolCredentialParameter


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**alg** | [**WebauthncoseCOSEAlgorithmIdentifier**](WebauthncoseCOSEAlgorithmIdentifier.md) |  | [optional] 
**type** | [**ProtocolCredentialType**](ProtocolCredentialType.md) |  | [optional] 

## Example

```python
from moddns.models.protocol_credential_parameter import ProtocolCredentialParameter

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolCredentialParameter from a JSON string
protocol_credential_parameter_instance = ProtocolCredentialParameter.from_json(json)
# print the JSON string representation of the object
print(ProtocolCredentialParameter.to_json())

# convert the object into a dict
protocol_credential_parameter_dict = protocol_credential_parameter_instance.to_dict()
# create an instance of ProtocolCredentialParameter from a dict
protocol_credential_parameter_from_dict = ProtocolCredentialParameter.from_dict(protocol_credential_parameter_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


