# ModelServicesSettings


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**blocked** | **List[str]** |  | [optional] 

## Example

```python
from moddns.models.model_services_settings import ModelServicesSettings

# TODO update the JSON string below
json = "{}"
# create an instance of ModelServicesSettings from a JSON string
model_services_settings_instance = ModelServicesSettings.from_json(json)
# print the JSON string representation of the object
print(ModelServicesSettings.to_json())

# convert the object into a dict
model_services_settings_dict = model_services_settings_instance.to_dict()
# create an instance of ModelServicesSettings from a dict
model_services_settings_from_dict = ModelServicesSettings.from_dict(model_services_settings_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


