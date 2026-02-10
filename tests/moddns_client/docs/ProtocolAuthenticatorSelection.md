# ProtocolAuthenticatorSelection


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**authenticator_attachment** | [**ProtocolAuthenticatorAttachment**](ProtocolAuthenticatorAttachment.md) | AuthenticatorAttachment If this member is present, eligible authenticators are filtered to only authenticators attached with the specified AuthenticatorAttachment enum. | [optional] 
**require_resident_key** | **bool** | RequireResidentKey this member describes the Relying Party&#39;s requirements regarding resident credentials. If the parameter is set to true, the authenticator MUST create a client-side-resident public key credential source when creating a public key credential. | [optional] 
**resident_key** | [**ProtocolResidentKeyRequirement**](ProtocolResidentKeyRequirement.md) | ResidentKey this member describes the Relying Party&#39;s requirements regarding resident credentials per Webauthn Level 2. | [optional] 
**user_verification** | [**ProtocolUserVerificationRequirement**](ProtocolUserVerificationRequirement.md) | UserVerification This member describes the Relying Party&#39;s requirements regarding user verification for the create() operation. Eligible authenticators are filtered to only those capable of satisfying this requirement. | [optional] 

## Example

```python
from moddns.models.protocol_authenticator_selection import ProtocolAuthenticatorSelection

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolAuthenticatorSelection from a JSON string
protocol_authenticator_selection_instance = ProtocolAuthenticatorSelection.from_json(json)
# print the JSON string representation of the object
print(ProtocolAuthenticatorSelection.to_json())

# convert the object into a dict
protocol_authenticator_selection_dict = protocol_authenticator_selection_instance.to_dict()
# create an instance of ProtocolAuthenticatorSelection from a dict
protocol_authenticator_selection_from_dict = ProtocolAuthenticatorSelection.from_dict(protocol_authenticator_selection_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


