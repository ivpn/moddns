# ModelCredential


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**id** | **str** |  | [optional] 

## Example

```python
from moddns.models.model_credential import ModelCredential

# TODO update the JSON string below
json = "{}"
# create an instance of ModelCredential from a JSON string
model_credential_instance = ModelCredential.from_json(json)
# print the JSON string representation of the object
print(ModelCredential.to_json())

# convert the object into a dict
model_credential_dict = model_credential_instance.to_dict()
# create an instance of ModelCredential from a dict
model_credential_from_dict = ModelCredential.from_dict(model_credential_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


