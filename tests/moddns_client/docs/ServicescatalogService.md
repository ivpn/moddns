# ServicescatalogService


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asns** | **List[int]** |  | [optional] 
**id** | **str** |  | [optional] 
**logo_key** | **str** |  | [optional] 
**name** | **str** |  | [optional] 

## Example

```python
from moddns.models.servicescatalog_service import ServicescatalogService

# TODO update the JSON string below
json = "{}"
# create an instance of ServicescatalogService from a JSON string
servicescatalog_service_instance = ServicescatalogService.from_json(json)
# print the JSON string representation of the object
print(ServicescatalogService.to_json())

# convert the object into a dict
servicescatalog_service_dict = servicescatalog_service_instance.to_dict()
# create an instance of ServicescatalogService from a dict
servicescatalog_service_from_dict = ServicescatalogService.from_dict(servicescatalog_service_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


