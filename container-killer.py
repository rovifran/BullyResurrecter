import os

CONTAINERS = {'s': 'server', 'p': 'processes', 'r': 'reviver'}

MENU = """
Containers:
s - server
p - processes
r - reviver

Enter container to kill: """

def kill_container(container_name):
    res = os.system(f"docker stop {container_name} -s 9")
    if res != 0:
        print(f"Failed to kill {container_name}")
    else:
        print(f"Killed {container_name}")

def main():
    try:
        while True:
            container = input(MENU)
            if len(container) == 0 or container[0] not in CONTAINERS:
                print("Invalid container")
                continue
            if len(container) == 1:
                kill_container(CONTAINERS[container[0]])
            else:
                container_type = CONTAINERS[container[0]]
                container_id = container[1]
                kill_container(f"{container_type}-{container_id}")
    except KeyboardInterrupt:
        print("Exiting...")

if __name__ == "__main__":
    main()
