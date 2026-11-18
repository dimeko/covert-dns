### covert-dns

`covert-dns` is a set od utils that let you send JSON data over DNS queries. It can be used as a covert communication channel.
The messages are encoded in base64 and split in chunks to be sent back and forth as DNS queries and DNS answers.

So if an object is encoded to a base64:
```bash
{
   "iam": "supposed to be encoded in base64 and the be split into small chunks",
   "test": "test test test test test"
}
# to
ewogICAiaWFtIjogInN1cHBvc2VkIHRvIGJlIGVuY29kZWQgaW4gYmFzZTY0IGFuZCB0aGUgYmUgc3BsaXQgaW50byBzbWFsbCBjaHVua3MiLAogICAidGVzdCI6ICJ0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QiCn0=
```

the message will be sent in chunks of 255 bytes (below chunks are less than 255 bytes):
```bash
ewogICAiaWFtIjogInN1cHBvc2VkIHRvIGJ
lIGVuY29kZWQgaW4gYmFzZTY0IGFuZCB0aG
UgYmUgc3BsaXQgaW50byBzbWFsbCBjaHVua
3MiLAogICAidGVzdCI6ICJ0ZXN0IHRlc3Qg
dGVzdCB0ZXN0IHRlc3QiCn0=
```

Then the server will concat the chunks and decode the base64 back in the type they share with the client.