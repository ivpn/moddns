# ModelAccountUpdate


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**operation** | **str** |  | 
**path** | **str** |  | 
**value** | **object** |  | 

## Example

```python
from moddns.models.model_account_update import ModelAccountUpdate

# TODO update the JSON string below
json = "{}"
# create an instance of ModelAccountUpdate from a JSON string
model_account_update_instance = ModelAccountUpdate.from_json(json)
# print the JSON string representation of the object
print(ModelAccountUpdate.to_json())

# convert the object into a dict
model_account_update_dict = model_account_update_instance.to_dict()
# create an instance of ModelAccountUpdate from a dict
model_account_update_from_dict = ModelAccountUpdate.from_dict(model_account_update_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


