# ModelDNSRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**dnssec** | **bool** |  | [optional] 
**domain** | **str** |  | [optional] 
**query_type** | **str** |  | [optional] 
**response_code** | **str** |  | [optional] 

## Example

```python
from moddns.models.model_dns_request import ModelDNSRequest

# TODO update the JSON string below
json = "{}"
# create an instance of ModelDNSRequest from a JSON string
model_dns_request_instance = ModelDNSRequest.from_json(json)
# print the JSON string representation of the object
print(ModelDNSRequest.to_json())

# convert the object into a dict
model_dns_request_dict = model_dns_request_instance.to_dict()
# create an instance of ModelDNSRequest from a dict
model_dns_request_from_dict = ModelDNSRequest.from_dict(model_dns_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


