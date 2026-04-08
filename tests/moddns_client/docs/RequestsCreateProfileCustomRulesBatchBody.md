# RequestsCreateProfileCustomRulesBatchBody


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**action** | **str** |  | 
**values** | **List[str]** |  | 

## Example

```python
from moddns.models.requests_create_profile_custom_rules_batch_body import RequestsCreateProfileCustomRulesBatchBody

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsCreateProfileCustomRulesBatchBody from a JSON string
requests_create_profile_custom_rules_batch_body_instance = RequestsCreateProfileCustomRulesBatchBody.from_json(json)
# print the JSON string representation of the object
print(RequestsCreateProfileCustomRulesBatchBody.to_json())

# convert the object into a dict
requests_create_profile_custom_rules_batch_body_dict = requests_create_profile_custom_rules_batch_body_instance.to_dict()
# create an instance of RequestsCreateProfileCustomRulesBatchBody from a dict
requests_create_profile_custom_rules_batch_body_from_dict = RequestsCreateProfileCustomRulesBatchBody.from_dict(requests_create_profile_custom_rules_batch_body_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


