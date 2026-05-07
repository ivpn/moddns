# RequestsCreateProfileCustomRuleBody


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**action** | **str** |  | 
**value** | **str** |  | 

## Example

```python
from moddns.models.requests_create_profile_custom_rule_body import RequestsCreateProfileCustomRuleBody

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsCreateProfileCustomRuleBody from a JSON string
requests_create_profile_custom_rule_body_instance = RequestsCreateProfileCustomRuleBody.from_json(json)
# print the JSON string representation of the object
print(RequestsCreateProfileCustomRuleBody.to_json())

# convert the object into a dict
requests_create_profile_custom_rule_body_dict = requests_create_profile_custom_rule_body_instance.to_dict()
# create an instance of RequestsCreateProfileCustomRuleBody from a dict
requests_create_profile_custom_rule_body_from_dict = RequestsCreateProfileCustomRuleBody.from_dict(requests_create_profile_custom_rule_body_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


