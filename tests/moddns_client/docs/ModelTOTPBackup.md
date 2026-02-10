# ModelTOTPBackup


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**backup_codes** | **List[str]** |  | [optional] 

## Example

```python
from moddns.models.model_totp_backup import ModelTOTPBackup

# TODO update the JSON string below
json = "{}"
# create an instance of ModelTOTPBackup from a JSON string
model_totp_backup_instance = ModelTOTPBackup.from_json(json)
# print the JSON string representation of the object
print(ModelTOTPBackup.to_json())

# convert the object into a dict
model_totp_backup_dict = model_totp_backup_instance.to_dict()
# create an instance of ModelTOTPBackup from a dict
model_totp_backup_from_dict = ModelTOTPBackup.from_dict(model_totp_backup_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


