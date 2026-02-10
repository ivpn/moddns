# ModelPrivacy


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**blocklists** | **List[str]** |  | [optional] 
**blocklists_subdomains_rule** | **str** |  | 
**custom_rules_subdomains_rule** | **str** |  | [optional] 
**default_rule** | **str** |  | 
**services** | [**ModelServicesSettings**](ModelServicesSettings.md) |  | [optional] 

## Example

```python
from moddns.models.model_privacy import ModelPrivacy

# TODO update the JSON string below
json = "{}"
# create an instance of ModelPrivacy from a JSON string
model_privacy_instance = ModelPrivacy.from_json(json)
# print the JSON string representation of the object
print(ModelPrivacy.to_json())

# convert the object into a dict
model_privacy_dict = model_privacy_instance.to_dict()
# create an instance of ModelPrivacy from a dict
model_privacy_from_dict = ModelPrivacy.from_dict(model_privacy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


