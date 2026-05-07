# ModelAccount


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**auth_methods** | **List[str]** |  | [optional] 
**email** | **str** |  | [optional] 
**email_verified** | **bool** |  | [optional] 
**error_reports_consent** | **bool** |  | [optional] 
**id** | **str** |  | [optional] 
**mfa** | [**ModelMFASettings**](ModelMFASettings.md) |  | [optional] 
**profiles** | **List[str]** |  | [optional] 
**queries** | **int** |  | [optional] 

## Example

```python
from moddns.models.model_account import ModelAccount

# TODO update the JSON string below
json = "{}"
# create an instance of ModelAccount from a JSON string
model_account_instance = ModelAccount.from_json(json)
# print the JSON string representation of the object
print(ModelAccount.to_json())

# convert the object into a dict
model_account_dict = model_account_instance.to_dict()
# create an instance of ModelAccount from a dict
model_account_from_dict = ModelAccount.from_dict(model_account_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


