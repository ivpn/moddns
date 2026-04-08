# ModelCustomRule


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**action** | **str** |  | 
**id** | **str** |  | 
**value** | **str** |  | 

## Example

```python
from moddns.models.model_custom_rule import ModelCustomRule

# TODO update the JSON string below
json = "{}"
# create an instance of ModelCustomRule from a JSON string
model_custom_rule_instance = ModelCustomRule.from_json(json)
# print the JSON string representation of the object
print(ModelCustomRule.to_json())

# convert the object into a dict
model_custom_rule_dict = model_custom_rule_instance.to_dict()
# create an instance of ModelCustomRule from a dict
model_custom_rule_from_dict = ModelCustomRule.from_dict(model_custom_rule_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


