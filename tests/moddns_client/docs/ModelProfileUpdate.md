# ModelProfileUpdate


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**operation** | **str** |  | 
**path** | **str** |  | 
**value** | **object** |  | 

## Example

```python
from moddns.models.model_profile_update import ModelProfileUpdate

# TODO update the JSON string below
json = "{}"
# create an instance of ModelProfileUpdate from a JSON string
model_profile_update_instance = ModelProfileUpdate.from_json(json)
# print the JSON string representation of the object
print(ModelProfileUpdate.to_json())

# convert the object into a dict
model_profile_update_dict = model_profile_update_instance.to_dict()
# create an instance of ModelProfileUpdate from a dict
model_profile_update_from_dict = ModelProfileUpdate.from_dict(model_profile_update_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


