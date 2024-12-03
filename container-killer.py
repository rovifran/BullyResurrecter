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
            container_name = [c for c in container]
            container_name[0] = CONTAINERS[container_name[0]]
            kill_container('-'.join(container_name))
    except KeyboardInterrupt:
        print("Exiting...")

if __name__ == "__main__":
    main()
