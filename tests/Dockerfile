FROM python:3.11.9

COPY ./certs /certs
COPY ./tests /tests/

WORKDIR /tests

RUN pip install --upgrade pip
RUN pip install -r requirements.txt

# Add the modDNS development root CA to the list of trusted certificates
RUN cat /certs/moddns_dev_development_CA.crt >> /usr/local/lib/python3.10/site-packages/certifi/cacert.pem

ENTRYPOINT ["bash"]
