# ServicescatalogCatalog


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**services** | [**List[ServicescatalogService]**](ServicescatalogService.md) |  | [optional] 

## Example

```python
from moddns.models.servicescatalog_catalog import ServicescatalogCatalog

# TODO update the JSON string below
json = "{}"
# create an instance of ServicescatalogCatalog from a JSON string
servicescatalog_catalog_instance = ServicescatalogCatalog.from_json(json)
# print the JSON string representation of the object
print(ServicescatalogCatalog.to_json())

# convert the object into a dict
servicescatalog_catalog_dict = servicescatalog_catalog_instance.to_dict()
# create an instance of ServicescatalogCatalog from a dict
servicescatalog_catalog_from_dict = ServicescatalogCatalog.from_dict(servicescatalog_catalog_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


