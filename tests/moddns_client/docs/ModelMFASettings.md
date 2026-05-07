# ModelMFASettings


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**totp** | [**ModelTotpSettings**](ModelTotpSettings.md) |  | [optional] 

## Example

```python
from moddns.models.model_mfa_settings import ModelMFASettings

# TODO update the JSON string below
json = "{}"
# create an instance of ModelMFASettings from a JSON string
model_mfa_settings_instance = ModelMFASettings.from_json(json)
# print the JSON string representation of the object
print(ModelMFASettings.to_json())

# convert the object into a dict
model_mfa_settings_dict = model_mfa_settings_instance.to_dict()
# create an instance of ModelMFASettings from a dict
model_mfa_settings_from_dict = ModelMFASettings.from_dict(model_mfa_settings_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


