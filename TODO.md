TODO
----

* write a client that asks for all known PDOs and exposes metrics on them
* write API for fan speed
* set TCP_NODELAY
* ~~replace log instances with method~~
* rethink tracing and create methods for wrapping info in spans
* rewrite read and write methods
    ```Java code for sending:
                   int length = transportMessage.operationByteArray.length + 34;
                  if (transportMessage.operationTypeByteArray != null) {
                      length += transportMessage.operationMessage.length;
                  }
                  ByteArrayOutputStream byteArrayOutputStream = new ByteArrayOutputStream(length);
                  DataOutputStream dataOutputStream = new DataOutputStream(byteArrayOutputStream);
                  try {
                      dataOutputStream.writeInt(length);
                      if (transportMessage.srcUUID != null) {
                          dataOutputStream.writeLong(transportMessage.srcUUID.getMostSignificantBits());
                          dataOutputStream.writeLong(transportMessage.srcUUID.getLeastSignificantBits());
                      } else {
                          dataOutputStream.writeLong(0);
                          dataOutputStream.writeLong(0);
                      }
                      if (transportMessage.dstUUID != null) {
                          dataOutputStream.writeLong(transportMessage.dstUUID.getMostSignificantBits());
                          dataOutputStream.writeLong(transportMessage.dstUUID.getLeastSignificantBits());
                      } else {
                          dataOutputStream.writeLong(0);
                          dataOutputStream.writeLong(0);
                      }
                      dataOutputStream.writeShort(transportMessage.operationByteArray.length);
                      dataOutputStream.write(transportMessage.operationByteArray);
                      if (transportMessage.operationMessage != null) {
                          dataOutputStream.write(transportMessage.operationMessage);
                      }
                      dataOutputStream.flush();
       
       Java code for reader:
           static /* synthetic */ void readMessage(CommunicationWithComfo socket) {
              byte[] srcUUID = new byte[16];
              byte[] dstUUID = new byte[16];
              while (socket.comfoSocket != null) {
                  try {
                      int length = socket.dataInputStream.readInt();
                      if (length < 1024) {
                          socket.dataInputStream.readFully(srcUUID);
                          socket.dataInputStream.readFully(dstUUID);
                          int operationLength = socket.dataInputStream.readUnsignedShort();
                          if (operationLength < 1024) {
                              byte[] operationBytes = new byte[operationLength];
                              socket.dataInputStream.readFully(operationBytes);
                              byte[] operationTypeBytes = null;
                              int operationTypeLength = (length - 34) - operationLength;
                              if (operationTypeLength > 0) {
                                  operationTypeBytes = new byte[operationTypeLength];
                                  socket.dataInputStream.readFully(operationTypeBytes);
                              }
                              TransportMessage transportMessage = new TransportMessage();
                              transportMessage.srcUUID = C0764g.m857a(srcUUID);
                              transportMessage.dstUUID = C0764g.m857a(dstUUID);
                              transportMessage.operationByteArray = operationBytes;
                              transportMessage.operationTypeByteArray = operationTypeBytes;
                              if (socket.f1245m != null) {
                                  socket.f1245m.sendMessage(socket.f1245m.obtainMessage(3, transportMessage));
                              }
                          } else {
                              logger.error("Invalid operationLength: ".concat(String.valueOf(operationLength)));
                              if (socket.f1245m != null) {
                                  socket.f1245m.sendMessage(socket.f1245m.obtainMessage(2));
                              }
                          }
                      } else {
                          logger.error("Invalid length: ".concat(String.valueOf(length)));
                          if (socket.f1245m != null) {
                              socket.f1245m.sendMessage(socket.f1245m.obtainMessage(2));
                          }
                      }
                  } catch (Exception e) {
```
