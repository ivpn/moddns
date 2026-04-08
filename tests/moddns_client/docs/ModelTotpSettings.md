# ModelTotpSettings


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**enabled** | **bool** | Indicates if TOTP is enabled. | [optional] 

## Example

```python
from moddns.models.model_totp_settings import ModelTotpSettings

# TODO update the JSON string below
json = "{}"
# create an instance of ModelTotpSettings from a JSON string
model_totp_settings_instance = ModelTotpSettings.from_json(json)
# print the JSON string representation of the object
print(ModelTotpSettings.to_json())

# convert the object into a dict
model_totp_settings_dict = model_totp_settings_instance.to_dict()
# create an instance of ModelTotpSettings from a dict
model_totp_settings_from_dict = ModelTotpSettings.from_dict(model_totp_settings_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


