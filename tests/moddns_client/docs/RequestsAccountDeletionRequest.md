# RequestsAccountDeletionRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**current_password** | **str** |  | [optional] 
**deletion_code** | **str** |  | 
**reauth_token** | **str** |  | [optional] 

## Example

```python
from moddns.models.requests_account_deletion_request import RequestsAccountDeletionRequest

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsAccountDeletionRequest from a JSON string
requests_account_deletion_request_instance = RequestsAccountDeletionRequest.from_json(json)
# print the JSON string representation of the object
print(RequestsAccountDeletionRequest.to_json())

# convert the object into a dict
requests_account_deletion_request_dict = requests_account_deletion_request_instance.to_dict()
# create an instance of RequestsAccountDeletionRequest from a dict
requests_account_deletion_request_from_dict = RequestsAccountDeletionRequest.from_dict(requests_account_deletion_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


