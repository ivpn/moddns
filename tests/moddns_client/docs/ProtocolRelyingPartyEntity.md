# ProtocolRelyingPartyEntity


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **str** | A unique identifier for the Relying Party entity, which sets the RP ID. | [optional] 
**name** | **str** | A human-palatable name for the entity. Its function depends on what the PublicKeyCredentialEntity represents:  When inherited by PublicKeyCredentialRpEntity it is a human-palatable identifier for the Relying Party, intended only for display. For example, \&quot;ACME Corporation\&quot;, \&quot;Wonderful Widgets, Inc.\&quot; or \&quot;ОАО Примертех\&quot;.  When inherited by PublicKeyCredentialUserEntity, it is a human-palatable identifier for a user account. It is intended only for display, i.e., aiding the user in determining the difference between user accounts with similar displayNames. For example, \&quot;alexm\&quot;, \&quot;alex.p.mueller@example.com\&quot; or \&quot;+14255551234\&quot;. | [optional] 

## Example

```python
from moddns.models.protocol_relying_party_entity import ProtocolRelyingPartyEntity

# TODO update the JSON string below
json = "{}"
# create an instance of ProtocolRelyingPartyEntity from a JSON string
protocol_relying_party_entity_instance = ProtocolRelyingPartyEntity.from_json(json)
# print the JSON string representation of the object
print(ProtocolRelyingPartyEntity.to_json())

# convert the object into a dict
protocol_relying_party_entity_dict = protocol_relying_party_entity_instance.to_dict()
# create an instance of ProtocolRelyingPartyEntity from a dict
protocol_relying_party_entity_from_dict = ProtocolRelyingPartyEntity.from_dict(protocol_relying_party_entity_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


