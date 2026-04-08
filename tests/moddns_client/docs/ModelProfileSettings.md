# ModelProfileSettings


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**advanced** | [**ModelAdvanced**](ModelAdvanced.md) |  | 
**custom_rules** | [**List[ModelCustomRule]**](ModelCustomRule.md) |  | [optional] 
**logs** | [**ModelLogsSettings**](ModelLogsSettings.md) |  | 
**privacy** | [**ModelPrivacy**](ModelPrivacy.md) |  | 
**profile_id** | **str** |  | 
**security** | [**ModelSecurity**](ModelSecurity.md) |  | 
**statistics** | [**ModelStatisticsSettings**](ModelStatisticsSettings.md) |  | 

## Example

```python
from moddns.models.model_profile_settings import ModelProfileSettings

# TODO update the JSON string below
json = "{}"
# create an instance of ModelProfileSettings from a JSON string
model_profile_settings_instance = ModelProfileSettings.from_json(json)
# print the JSON string representation of the object
print(ModelProfileSettings.to_json())

# convert the object into a dict
model_profile_settings_dict = model_profile_settings_instance.to_dict()
# create an instance of ModelProfileSettings from a dict
model_profile_settings_from_dict = ModelProfileSettings.from_dict(model_profile_settings_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


