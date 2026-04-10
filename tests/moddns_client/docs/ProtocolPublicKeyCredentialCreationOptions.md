# ProtocolPublicKeyCredentialCreationOptions


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**attestation** | [**ProtocolConveyancePreference**](ProtocolConveyancePreference.md) |  | [optional] 
**attestation_formats** | [**List[ProtocolAttestationFormat]**](ProtocolAttestationFormat.md) |  | [optional] 
**authenticator_selection** | [**ProtocolAuthenticatorSelection**](ProtocolAuthenticatorSelection.md) |  | [optional] 
**challenge** | **List[int]** |  | [optional] 
**exclude_credentials** | [**List[ProtocolCredentialDescriptor]**](ProtocolCredentialDescriptor.md) |  | [optional] 
**extensions** | **Dict[str, object]** |  | [optional] 
**hints** | [**List[ProtocolPublicKeyCredentialHints]**](ProtocolPublicKeyCredentialHints.md) |  | [optional] 
**pub_key_cred_params** | [**List[ProtocolCredentialParameter]**](ProtocolCredentialParameter.md) |  | [optional] 
**rp** | [**ProtocolRelyingPartyEntity**](ProtocolRelyingPartyEntity.md) |  | [optional] 
**timeout** | **int** |  | [optional] 
**user** | [**ProtocolUserEntity**](ProtocolUserEntity.md) |  | [optional] 

## Example

```python
from moddns.models.protocol_public_key_credential_creation_options import ProtocolPublicKeyCredentialCreationOptions

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolPublicKeyCredentialCreationOptions from a JSON string
protocol_public_key_credential_creation_options_instance = ProtocolPublicKeyCredentialCreationOptions.from_json(json)
# print the JSON string representation of the object
print(ProtocolPublicKeyCredentialCreationOptions.to_json())

# convert the object into a dict
protocol_public_key_credential_creation_options_dict = protocol_public_key_credential_creation_options_instance.to_dict()
# create an instance of ProtocolPublicKeyCredentialCreationOptions from a dict
protocol_public_key_credential_creation_options_from_dict = ProtocolPublicKeyCredentialCreationOptions.from_dict(protocol_public_key_credential_creation_options_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


