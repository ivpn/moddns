# ModelQueryLog


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**client_ip** | **str** |  | [optional] 
**device_id** | **str** |  | [optional] 
**dns_request** | [**ModelDNSRequest**](ModelDNSRequest.md) |  | [optional] 
**id** | **str** |  | [optional] 
**profile_id** | **str** |  | [optional] 
**protocol** | **str** |  | [optional] 
**reasons** | **List[str]** |  | [optional] 
**status** | **str** |  | [optional] 
**timestamp** | **str** |  | [optional] 

## Example

```python
from moddns.models.model_query_log import ModelQueryLog

# TODO update the JSON string below
json = "{}"
# create an instance of ModelQueryLog from a JSON string
model_query_log_instance = ModelQueryLog.from_json(json)
# print the JSON string representation of the object
print(ModelQueryLog.to_json())

# convert the object into a dict
model_query_log_dict = model_query_log_instance.to_dict()
# create an instance of ModelQueryLog from a dict
model_query_log_from_dict = ModelQueryLog.from_dict(model_query_log_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


