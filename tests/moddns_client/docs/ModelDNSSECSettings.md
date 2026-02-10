# ModelDNSSECSettings


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**enabled** | **bool** |  | 
**send_do_bit** | **bool** |  | 

## Example

```python
from moddns.models.model_dnssec_settings import ModelDNSSECSettings

# TODO update the JSON string below
json = "{}"
# create an instance of ModelDNSSECSettings from a JSON string
model_dnssec_settings_instance = ModelDNSSECSettings.from_json(json)
# print the JSON string representation of the object
print(ModelDNSSECSettings.to_json())

# convert the object into a dict
model_dnssec_settings_dict = model_dnssec_settings_instance.to_dict()
# create an instance of ModelDNSSECSettings from a dict
model_dnssec_settings_from_dict = ModelDNSSECSettings.from_dict(model_dnssec_settings_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


