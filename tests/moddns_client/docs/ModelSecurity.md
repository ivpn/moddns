# ModelSecurity


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**dnssec** | [**ModelDNSSECSettings**](ModelDNSSECSettings.md) |  | 

## Example

```python
from moddns.models.model_security import ModelSecurity

# TODO update the JSON string below
json = "{}"
# create an instance of ModelSecurity from a JSON string
model_security_instance = ModelSecurity.from_json(json)
# print the JSON string representation of the object
print(ModelSecurity.to_json())

# convert the object into a dict
model_security_dict = model_security_instance.to_dict()
# create an instance of ModelSecurity from a dict
model_security_from_dict = ModelSecurity.from_dict(model_security_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


