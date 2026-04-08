# ModelTOTPNew


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**account** | **str** |  | [optional] 
**secret** | **str** |  | [optional] 
**uri** | **str** |  | [optional] 

## Example

```python
from moddns.models.model_totp_new import ModelTOTPNew

# TODO update the JSON string below
json = "{}"
# create an instance of ModelTOTPNew from a JSON string
model_totp_new_instance = ModelTOTPNew.from_json(json)
# print the JSON string representation of the object
print(ModelTOTPNew.to_json())

# convert the object into a dict
model_totp_new_dict = model_totp_new_instance.to_dict()
# create an instance of ModelTOTPNew from a dict
model_totp_new_from_dict = ModelTOTPNew.from_dict(model_totp_new_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


