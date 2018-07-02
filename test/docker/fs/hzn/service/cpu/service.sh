#!/bin/sh

# Example Blue Horizon "Microservice" that returns CPU percentage
# Note: requires gawk and bc to be installed.

# Get CPU usage from /proc/stat. This works even inside a docker container (gets the host total CPU usage).
getCpuFromProc() {
        # Get 2 samples from the cpu line of /proc/stat, then do the math on the deltas.
        # See the explanation at https://github.com/Leo-G/DevopsWiki/wiki/How-Linux-CPU-Usage-Time-and-Percentage-is-calculated
        line1=$(grep -iE '^cpu ' /proc/stat)
        total1=$(echo "$line1" | gawk '{print $2+$3+$4+$5+$6+$7+$8+$9}')
        idle1=$(echo "$line1" | gawk '{print $5+$6}')
        # echo "total1=$total1, idle1=$idle1"
        sleep 1
        line2=$(grep -iE '^cpu ' /proc/stat)
        total2=$(echo "$line2" | gawk '{print $2+$3+$4+$5+$6+$7+$8+$9}')
        idle2=$(echo "$line2" | gawk '{print $5+$6}')
        # echo "total2=$total2, idle2=$idle2"
        cpuu=$(echo "scale=2; 100 * (($total2 - $total1) - ($idle2 - $idle1)) / ($total2 - $total1)" | bc)
        needPrefix=$(echo "${cpuu}<1.0" | bc)
        if [ "${needPrefix}" = "1" ]; then
                rcpuu="0${cpuu}"
        else
                rcpuu=${cpuu}
        fi
        echo $rcpuu
}

# Get the currect CPU consumption, then construct the HTTP response message
CPU=$(getCpuFromProc)
HEADERS="Content-Type: text/html; charset=ISO-8859-1"
BODY="{\"cpu\":${CPU}}"
HTTP="HTTP/1.1 200 OK\r\n${HEADERS}\r\n\r\n${BODY}\r\n"

# Emit the HTTP response
echo -en $HTTP

