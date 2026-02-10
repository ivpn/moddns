# ProtocolUserEntity


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**display_name** | **str** | A human-palatable name for the user account, intended only for display. For example, \&quot;Alex P. Müller\&quot; or \&quot;田中 倫\&quot;. The Relying Party SHOULD let the user choose this, and SHOULD NOT restrict the choice more than necessary. | [optional] 
**id** | **object** | ID is the user handle of the user account entity. To ensure secure operation, authentication and authorization decisions MUST be made on the basis of this id member, not the displayName nor name members. See Section 6.1 of [RFC8266](https://www.w3.org/TR/webauthn/#biblio-rfc8266). | [optional] 
**name** | **str** | A human-palatable name for the entity. Its function depends on what the PublicKeyCredentialEntity represents:  When inherited by PublicKeyCredentialRpEntity it is a human-palatable identifier for the Relying Party, intended only for display. For example, \&quot;ACME Corporation\&quot;, \&quot;Wonderful Widgets, Inc.\&quot; or \&quot;ОАО Примертех\&quot;.  When inherited by PublicKeyCredentialUserEntity, it is a human-palatable identifier for a user account. It is intended only for display, i.e., aiding the user in determining the difference between user accounts with similar displayNames. For example, \&quot;alexm\&quot;, \&quot;alex.p.mueller@example.com\&quot; or \&quot;+14255551234\&quot;. | [optional] 

## Example

```python
from moddns.models.protocol_user_entity import ProtocolUserEntity

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolUserEntity from a JSON string
protocol_user_entity_instance = ProtocolUserEntity.from_json(json)
# print the JSON string representation of the object
print(ProtocolUserEntity.to_json())

# convert the object into a dict
protocol_user_entity_dict = protocol_user_entity_instance.to_dict()
# create an instance of ProtocolUserEntity from a dict
protocol_user_entity_from_dict = ProtocolUserEntity.from_dict(protocol_user_entity_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


