# service-maker
Simple library for helping with turning executable into system service

## usage

Example usage with *install* flag:

```
install = flag.Bool("install", false, "install service in os")

flag.Parse()

if *install {
		service := servicemaker.ServiceMaker{
			User:               "runasuser",
			ServicePath:        "/etc/systemd/system/servicename.service",
			ServiceDescription: "service description here",
			ExecDir:            "/srv/servicedir",
			ExecName:           "serviceexec",
		}
		err := service.InstallService()
		if err != nil {
			panic(err)
		} else {
			fmt.Println("service installed!")
			return
		}
	}
```
