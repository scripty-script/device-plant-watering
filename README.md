# Commands

### Manual provision

#### Example

```
pws migrate
pws auth -usr "rpi-device" -pwd "rpi-device" -host "192.168.1.61"
```

or

```
pws migrate
go run . auth  -usr "rpi-device" -pwd "rpi-device" -host "192.168.1.61"
```

# Build File

### Linux

```
GOOS=linux GOARCH=arm64 go build -o build/linux/pws
```

### Windows

```
GOOS=windows GOARCH=arm64 go build -o build/windows/pws
GOOS=windows GOARCH=amd64 go build -o build/windows/pws.exe
```

# Transfer build file to test device

### Step 1 - Your local machine

```
scp ./build/linux/pws rpi@192.168.1.61:~
```

### Step 2 - Move the app

```
sudo mv pws /usr/local/bin
```

### Step 3 - Permission

```
chmod 777 /usr/local/bin/pws
```

```
scp ./pws-service.conf rpi@192.168.1.61:~
sudo mv pws-service.conf /etc/supervisor/conf.d

sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start "pws-service:*"
```
