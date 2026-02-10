# RequestsSubscriptionReq


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**active_until** | **str** |  | 
**id** | **str** | ID is the external Subscription ID (UUIDv4) | 

## Example

```python
from moddns.models.requests_subscription_req import RequestsSubscriptionReq

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsSubscriptionReq from a JSON string
requests_subscription_req_instance = RequestsSubscriptionReq.from_json(json)
# print the JSON string representation of the object
print(RequestsSubscriptionReq.to_json())

# convert the object into a dict
requests_subscription_req_dict = requests_subscription_req_instance.to_dict()
# create an instance of RequestsSubscriptionReq from a dict
requests_subscription_req_from_dict = RequestsSubscriptionReq.from_dict(requests_subscription_req_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


