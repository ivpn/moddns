# ModelLogsSettings


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**enabled** | **bool** |  | 
**log_clients_ips** | **bool** |  | 
**log_domains** | **bool** |  | 
**retention** | [**ModelRetention**](ModelRetention.md) |  | 

## Example

```python
from moddns.models.model_logs_settings import ModelLogsSettings

# TODO update the JSON string below
json = "{}"
# create an instance of ModelLogsSettings from a JSON string
model_logs_settings_instance = ModelLogsSettings.from_json(json)
# print the JSON string representation of the object
print(ModelLogsSettings.to_json())

# convert the object into a dict
model_logs_settings_dict = model_logs_settings_instance.to_dict()
# create an instance of ModelLogsSettings from a dict
model_logs_settings_from_dict = ModelLogsSettings.from_dict(model_logs_settings_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


