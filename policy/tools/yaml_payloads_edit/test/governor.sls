governor:
  prod: |-
    [
        {
            "matchgroups": [
                [
                    {
                        "attr": "is_bandwidth_test_enabled",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "arm"
                    }
                ]
            ],
            "deployment": "{\"netspeed5\":{\"image\":\"summit.hovitos.engineering/armhf/netspeed5:volcano\",\"links\":[\"pitcherd\"],\"environment\":[\"DEPL_ENV=prod\"],\"restart\":\"always\"},\"pitcherd\":{\"image\":\"summit.hovitos.engineering/armhf/pitcherd:volcano\",\"environment\":[\"MTN_CATCHERS=https://catcher.bluehorizon.network:443\",\"MTN_PITCHERD_PORT=8081\"],\"restart\":\"always\"}}",
            "deployment_signature": "BASX86WLTeqZitL1QZ0uoXXlpceZBJd/r4vAImpASMQKGXj0RMn3fWzncKYFeS8mfXNIeQjezttqHBSv+QsXPa9gZ27ZEbOZ0LKB+VoHcuJ0ck0u9xRrNR+k9OJv9KCj5clXMhY/WLwzBWBJ61DCHDdV8HAZ1HXpxYUyj6ZQGyO/zJim/vTyCtqflBzSCS6Lrqqn7Dy39IlXUYQUd1fA43Ythc1NPc8RtfpYYdCUbnZA7SeWtMbrGDr5Zaph0Ckit3/ABlat9l6RitewvsSRvie8OWn6nfBbsnIl2QIpjjdtY9ygP2h/HqhMsIB42LyoM9S2duLARxZBp9XuW7+Qn4K8DsvGdTECzG43NswRWQ+3OgEz0xWVcCj9sDMMzCK5RzB0gR+G5TwRgNmgUzts2gubUw8WCBjzbwjMTA1Q5WKIMp5UhT10juK2ktQI4BS+TcAep5vjIh3Jhy0uQjHo5cLcBnbR5sxrPDc2lYTTFNKnjq2QiRJJ88Iw/o0gmL34YH6UWfCzUG8B3NVi4dg5ukz15MVKg5sw7iiRH1CL6ZRkhmxt7g5gEN8DR6XAyNSRs83ibLaAEmqEGPlMEpCh2q1rrXMhCnr64z0Odwc0zr6o0ntWj3Qu4TZhT+1IsnL8XGnA7mqRl/lP+k4DveLUbAyZ0xKCMsut5bK6eTlY6tI=",
            "torrent": {
                "url": "https://images.bluehorizon.network/3edd6b25d834fa413992323c1d466449008293be.torrent",
                "images": [
                  {
                    "file": "9dde5aa59536b291bcdd0ff67b59be08e9150f37.tar.gz",
                    "signature": "QN4rEBqtZb3ogutfLLN1pm5LuyPskRd5MEWWFTzHQIMz24wNIt4OtfZk9xk/Ly0AvO5HA8vZZ/ZBMuw9FptrLt9pH/z9S1nscT+1ASmJQFVSEVYKCx9d1VTEXT2UOCgFmXIRAAhOlc4R8nTwIhUyPJMfxjmqwYi/wdk53yn3+N7YpyOo8FzneW73qhJlR9YyRRcxY+BY+RmqvbvxF8kw3712OA5vyLJb3OhTf1oA53/QJDCqbq57cec3Gt0W7dzuHkrF03h1UiDnGDZL5X98jAfUZpdagicy9jTNpgE3Nz+a2eLsVnGQ51cfBzWmLFhOXH1DCwI5NuIFFKYAcbDkOP1+ZqzoIu1FNkaPCVqWJrtt4Ri/m9SqUicgLfvHSl3raqSYH5Dj/PLIP5n1IHEPeC2PbZR5URpV8Rs0g9QxXAB22JPfM7sWQH7GHA1CkZfL/DrrdY8p2IDrLYiNxzlie9rjBajFSK8sye9Az8FUBXBO8ZDICxibMc5aOcyDXWV3e+i+Fp7sAZ+hWpKpXN0qjLPffVee3oZJ5sXeuVNdLalUKbIudIztELmH0/MJBRWC0t8ml9G3srVzRWa6b3Iy+ZJqyVRvlgA3QHu1VaNeYCPM9HHxaAsNTmCHp1Fp4NGJavvadx6M6L+bNGl8AFUR+Nl1lGDDxEm9Z2qnk6eIS/E="
                  },
                  {
                    "file": "d36ed2876921c43e10cfc0f2bfea866d876281f7.tar.gz",
                    "signature": "s+VUthtao7xUGPkFTbGvGiFbGxSypFLKIvUDdn5zgeeFpgx5K8buCExU4djapqqzAlxF4KJ4TuzHqQ6/eAN4+npSclylCJd5un4oVd1FWq5Eem0R/ai0N7M4zuZjniTo3IVNSDzpxECX43QG8JAqk2aAql1tiZhjuDy44pkFo5ALdLAuD4pneCtvG1CFwbDDPSBvtJ4ycJ1MlwzW9AJEJ+Pp+pX7MVGekA3sFwcgVfSZML+l4+Sk7/MIXgm+iQyvb4MvWHLb/litGFQ+82DsxQ4m+DUqR6Z4djO+q4VJHyWX+LIb8FC+JmFG/VIA1P5+StijCeo3NGnr7TYXgk+LEVSrHqNlwhr7qdCQfVM9VpsrFDflEeFMqTVYWBU8klZF1HzQsa3z+X/y0jRwssHSVa4wsDfdBhWN+7ijzhz5G3JK/qisISGV9f0wfVZGQm8AcJcp+X0Zbhag61VzHcTUj6W0AddfjhNuGeaLll0mJq0B0dR9dy/WMa7FEoAfeRu569h8NeFPhc9Qr+WN4V8+y3k7zpSusrSiHgtSY2kJJUlhegg/Sqd6TGd1Vhg0RFEovjodlx/zUI5rzVOb/e50+1USQwnPuwjeoFXZggRrFhLZa3P0rSdOelroXoEZFh0aw6uGfCkKf5qf7rUPeTEcgKNZy6/5h+CS7bgJLUdZG2g="
                  }
                ]
            }
        },
        {
            "matchgroups": [
                [
                    {
                        "attr": "pws",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "arm"
                    }
                ]
            ],
            "deployment": "{\"culex\":{\"image\":\"summit.hovitos.engineering/armhf/culex:volcano\",\"environment\":[\"MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod\"],\"restart\":\"always\"},\"fwweather\":{\"image\":\"summit.hovitos.engineering/armhf/fwweather:volcano\",\"privileged\":true,\"environment\":[\"MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod\"],\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"links\":[\"culex\"],\"restart\":\"always\"},\"eaweather\":{\"image\":\"summit.hovitos.engineering/armhf/eaweather:volcano\",\"environment\":[\"MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod\",\"DEPL_ENV=prod\"],\"links\":[\"culex\"],\"restart\":\"always\"}}",
            "deployment_signature": "l1RkTOrHdRNfLJC1b4RohSfOHqYdr4Sd8A6d0wxYKd1kPSCBu4qhMRryNJNR9KCViY51Rz5p0bSmlQ+JzKjJhTXeHSPwQnYeLTLOc9Mix5LPwRk/lRBlp7j/a6wkyvDSds7O2AUJTynSgj0PZQbH7F6aT7FylcBgPHNyXN+ksrCK2ghpWb/XaoxVVPzMdDTNxN9blUbWwQXLGDv8oZG6cnMUhPcJQFHJFxYMAy3VdEmKq+x2sDBGfPo+GCrQCH2IwjeTzitvwu413ejRxjgJlvWICPeNjhg79poQzLTML2iONQJr/wNY4me60MUrRGkVG9eBJq0u9P8cEu+5n71NP/QyBFOY6vkJdB+cCXknlnx1VUk3p3EY0r4ICwB40CevSNPmS/rgPtAPfbvZxdni0cN+yfM8vAPgwRUhrz6y+mvRHtG/kXa+QJUE7JRjIk7tXi5VLar37w5kYWC+AkpMeC6cDvyCnZO7TiOhpxPy71SmUbZPVWuDM5m7SF4ySm5I1d/LyOcFdgcKIoIBOaFNBQ9QPPUjLdWEmVjRpG3ccgiuBdEsY+b66N3AyBD5fTrBFduepUgsq4DUvZKVh8H66yVGw44PmIWFJiHxrlYQ5JYbXJ+R1bS8xB5mLnVO/DChWrCcUbtqEGNYw1mvcz4+2BQ5L5nYLxrj80tXYKJ3L1g=",
            "torrent": {
                  "url": "https://images.bluehorizon.network/4cc266a4c16a951a187f996c8bd6ff8f46725db2.torrent",
                  "images": [
                    {
                      "file": "327e911895cbf66fce06452caeeb2d6ded10e577.tar.gz",
                      "signature": "NhOC1yAnGkdcfX33WqlOk3OHr6+OaVuGQxgewR8pqVVb0NbTBzDZ0dpn5Kz/8P2qjWu82N+3l724EJe2kaGM2kDLnlobZfnFMgmE/sAY3YDIUEBiSap3F5TgGirBavlEEFlIIwqnWhNdPlxxk5OtDCEhQ24Wj8hnBLJpe+coWcFxvmlqsIFA8saL0emGEDyXqX6zV3KrxqYNPq5GHP/OtuB0Vs9bopFAX+emHEVRXxXTEQwcL9T1BPkflpoEj6L/NkV6jkiDVhoPbKRSzV785WK8DA5s5hjRZ7JH82d1W1NNJEcDRy9tuEzGS4IveMjajrc1FdTPiuTC//GMWh0sJu76MchXaoQxjX7G7R2D1jyVcH65vxxlMlkejtXeo0sx8SzMZ201hhveZ216AwBvYmwuHS/pvVA2dbl33UKq6S2D5Xnre66/WESS3scvRM5cTlKtl7TlnxcYT7ootL8KRHzWAtkgXTAYtxI4VWr4NIMzoXT6/gCHHeupd2lra9P7V/aonfJdCvNuLMRddGijaXsrkibB08cHg4gC+qUEkb5FvgInrJz/OvVvUYfmCWnPlUGcnlU4UfQEZXodgOMzHmanccEoOSth23zaIqdMTnMPABTsvEiWEaw6AB8yf7yEapIPjBgRXovR5BTczVRInoVW239WrTny0K61BZkPQb0="
                    },
                    {
                      "file": "7dd1e8a452fe75014222866b1691ba259a16e241.tar.gz",
                      "signature": "kDeMIv5b9CJ4Pu4Ja8fxOD15gaY3QgdXLw1/GOhQp0CMhuyT+ew/lzFbtnG4Sp9u0cFGsmnUlnNHJUv4gfye1mn7Wf8PxWAewi3jBFqN9tNwUZqnLNcz5ob4IZWFRTMGPv75Dmgo5xbbMP1Y5qTnBsCffns3KdFN9NNLj0hkpVA/CpwFGuOgYCvN3scwOICJ89gzh6HosTdeGi0KHqqaI4/pyB22Umchl1mylBDzDHDjluEvR8p4wKnKurWz5DoyUcTWz2NU2gwMXDuI1VTbAErQisdrT9H32nCuqUaNpvgXglzSFv0Ddsxyy93pc1lBmev6KTLYapefsFaRJm7W8lmF0CHB2jP3THRKWpMJ8JUVJiZl2U+Oa/Vu3hBS89NgRr2xNLvR6pW6azQx03gjbnRlLZTWalLiH7LNMFxh1KUN0sUNcyAHC3QpHo/gIEMuhf57Cda6isT7QhK5bNoiRoJGmKVaYYqsdly74MZL2UzG3bg/BEIfHSxkKSGYDKoYe+KeABdcwdBSbVUX6MIWu7AgBu2jFFIIGF2nyej05dSOV+08wO0hkoI7Yyb/ovMD1c6a2brX7ysFN2+Xp96z1PHHP9Q+Rfhj6YfcLsPfBmXnBh3W6Ey9Iu60dm/X8mOTyVKrjIXLJplL6hCxXNgGHfMtfqTKhVtDMmwjxI+4VyA="
                    },
                    {
                      "file": "b6bd85af3e9d99084fa2759c032b3ba3965c9303.tar.gz",
                      "signature": "wxbNjeFz5Rjs83HVhLl0iFU4jFGQsTAXYLQq4a5rpFfVeuhXJ7SdqOu6uYbSrH4MdlC5WFQlrm/mBBUmsm3weVqv+mrZYg40SCyfY9XurOZkO2aewxWhfv6imq9gj3VM6AiR5h6Ci0UP0YWDiTYfxbdufhLxNFWBi/KgroErufcX1nIGgEvGq+t+DjBBAHeRTvJCjQmRbyvObYwcteXfvoQ6B6YWMLnDuKFBRUpJkbuh1Yu7QqUIflpf8oS2n/f3RiCeQPJd4IH2na40HwmM4PPEVOvl6kap7w3XBapvsKhg54o2ZCrJbuzG7NS6TFUztdSMKqdY8I/r2CoWAwTLv1eIl6FJQYH/xTGPS/FBcrgdiNnGRj+RatsUqYvTc8vuvXaNyBMRoj6g8moEKGoQZiwxEQiO0RGMjoDYHgvQHB82kBcxmEyV4hIKro582Nzfvp4rZEkoLll2Hn7Ikji5vOp31upXbsi5mgf7Z6dAysVfDsD58BYD3JejYeVSqYb4MR4GMeuhc7p2MElS0x4xKyz7plMAsBLbDjH3k6fSIW8BtGt2t0NFOD9rYAo8bvHp0pTaDNFaj/KUJD3D0mpnwyvj8DVkc4/Dgl9vaoAcuPm6q7K/drMRdf2YGx4ryhnxn3E3VYh8Oroo+hWFUmT3P7ucKVle3QObC9D4uzC/7XQ="
                    }
                  ]
            }
        },
        {
            "matchgroups": [
                [
                    {
                        "attr": "is_bandwidth_test_enabled",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "amd64"
                    }
                ]
            ],
            "deployment": "{\"netspeed5\":{\"image\":\"summit.hovitos.engineering/x86/netspeed5:volcano\",\"links\":[\"pitcherd\"],\"restart\":\"always\"},\"pitcherd\":{\"image\":\"summit.hovitos.engineering/x86/pitcherd:volcano\",\"environment\":[\"MTN_CATCHERS=https://catcher.bluehorizon.network:443\",\"MTN_PITCHERD_PORT=8081\"],\"restart\":\"always\"}}",
            "deployment_signature": "sfsxqGjqUZxsEHXJC2cwSbjr/51KLAbU6hRs2aoX30r798i+MIXpdxZtC4OSqSsURht506G+4oB/+9Kil317sIpF2it5IXBRYLc704AGIcgyh2cbKd9JTniUUNwFOFaco35igjNVdjaGhuRGtpp63ONKaNWEkYEAXeS74FbDzAMbF2GRwICVG/C5xronrQUjEVoKFATh9HwywgmvNGLhBOfzHOfAaZ2YYPaEZyDNV6Gvj83YbrFhBF4iOnd1wRsRC97YDd+6snt/iPSnViKOAKhIxzcS/SiUwyHPtaAOoMK+c3IjCgy6HKCt2Hau6D7k5LS4yjbFi/R5/kogk38Qf00auGBMWTVkSnhUwyY+kk1g2IyxT3p0NuHLGEj//Gr8tSPFF4M8vyENWRB9RSIXl4PoDBBjuuqEHTacoDwm6tovflpr/Hxag2DDMsOF+9o4JwxV0j9j2xrzu7jTUbHCxjllN70ay10id5UCwTU5Nu6d98SRGmPqC1YWtFuKb8PPq+9jj8wNGQAFuCrs24iEYyrXWpguzOavCANiWk2LLKnHVBeUQSXf4TpNyuhEn8HsTbvXjYi1AnatOSvcag38D7YICfGkdZxcjxvDBjJ6M6uIPbGlTFFemJ3eJOBJdyoDumkUkVqy/dHxAJM4MmLSlPTxjEOKGpWE7RMoE9n3DRw=",
            "torrent": {
                "url": "https://images.bluehorizon.network/9e07c992bc709bf24d55d2e215cc1e431484dfd4.torrent",
                "images": [
                    {
                        "file": "0544b73956f4333b314ef28c3c7931277225bcf0.tar.gz",
                        "signature": "sDT6wVc3wbIF2uAFGO4l2jIM02dcsEafQRmJWter1F0ItP+SUNym5MPEyoA8lp+9GXmFhzQs8PTXJHo5u5m4Msqa5PeSrICnxdqlr0sQMUxMn/ollJs4qIpk7VUMJTa68v2AUojxx8U36WMKvxDUGGaLnibSBpYISv0a6+cu/fHUlslE/zHvWRV6MP5Np8F7w19+Ed6G7OOhLMezQtqZes56D+OHMnx6xZcB7vcKWIubm2jzzUBl3BLSB42M8mBCVFvFJZeQ0HXtxSj+VfJ3hZb6yFmLt0ggWSgJdxL3WPUM1YwqQ+7ELb7PYczyf79D8LL9zM7FxM2EupTs90ys6V0pHZLhP+20DIEfccEPKMA4y0nRdqe9o7aLQwGB7+b6LJGzD8sKvtxqua8+wXx9x/szU+EwQXNBKA9bYnUJqpz8Qrj3pTQjTB9Ap1bPBpTCc9yUrBfUe/k+/tkILUzc8VWBWwHIpa3GfOVwX2uTr9q3oANlu5HOOiZq+WCYQNxHu/LT7Fm4QzeAU13TsGWTHd/piN5m66UNyEmTGGsIanf+lN0CiEyeJzJ2Es5SFNPbtyvsKrYZLssPCM1/n6vlHWKVoagW/fzGzAhCfxWGnYVxI7A6SO1OruBLHDro8SFvrRx3jxWHgNgcZR9FxuFpqTBP/BPOaLR9RN/SFgC8sZ0="
                    },
                    {
                        "file": "2d1c12d4a33c5ede7be47d0fded289b779b3bc73.tar.gz",
                        "signature": "cBSTiYqbSHSlUlK5zLAB9LbWJ3RgMBE9sMFgWATN9WTL8egwtGvpHv6YVFfVl3fg1tBt2v4Kry/xutDVPy6qNI7iicZX3v49m1MwqpmyQfV5cznsG5E1ekbYVrg4vcd4+RMzFIAInxsqS7KruI8MvXIJMcYlqE6eg5GIGYR6KVr0fC1yVqr3+8PXccNmPT5oTf7lveoAfOT9rhIwQSoSgIhLEHb7N3B0v02WPKJMRzxcRAkNLoDYZtKahi5VTudfVODWtjGEKy47U/f2fv4HHwRysuETzKqhHWToJHbdXSBHxagbRfyMPoXim2Om5Mn7CQP2siNsSXNpxPmYtIYmgxU/K1PcmhdfMiWW9KJpYgqda9Jqdm047jV/NSZSPCffcpIVyc5HgqLrMZs7NSt/f2Z9gUemSwSClRZ1aGD6wxUXBRShSnu9/jEcWslWrWmXJvzG8Lbl6t8rMDBr7X7pnJAm2mZfQxR0KJddfSQRX+WGFMsa16M0/MT9K90fL0X+RJFGyBdeOzlVDm+nmgUH8VImmZE069lfsoNAgKTwsGCz+tF8Z/K+7DLliQUyTI3al3cHc/7yUjdywUreIVrbTjZhepG26OeBHn9W820+NsVcouFi30r+TObeO8G7fN9Met/i76TbkgeU12HNJCZMRpTXIxjxFFSOgPzA8KSCudE="
                    }
                ]
            }
        },
        {
            "matchgroups": [
                [
                    {
                        "attr": "sdr",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "arm"
                    }
                ]
            ],
            "deployment": "{\"rtlsdr\":{\"image\":\"summit.hovitos.engineering/armhf/rtlsdr:volcano\",\"privileged\":true,\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"restart\":\"always\"},\"pitcherd\":{\"image\":\"summit.hovitos.engineering/armhf/pitcherd:volcano\",\"environment\":[\"MTN_CATCHERS=https://catcher.bluehorizon.network:443\",\"MTN_PITCHERD_PORT=8081\"],\"restart\":\"always\"},\"apollo\":{\"image\":\"summit.hovitos.engineering/armhf/apollo:volcano\",\"links\":[\"rtlsdr\",\"pitcherd\"],\"environment\":[\"CATCHER_HOST=catcher.bluehorizon.network\",\"CATCHER_PORT=443\",\"DUMP1090_HOST=bluehorizon.network\",\"JOBS_HOST=bluehorizon.network\",\"DEPL_ENV=prod\"],\"restart\":\"always\"}}",
            "deployment_signature": "jMfb8XTE2EJK0kQvY0GzjzaHjAjm/EnpWm2mJiDH9ErHQiCnvohPAwuQ0qpUxhNpPxVdEd31wRPtwbLEfluuosCUo+eed+Xfu3ziCUx9xTxLuGlKyvvDYkNp6vPaSqd/l3F6v9pRsNuu7l0Pa0gjISRBufU9TuMUFZlWCZaroIxqkYKxz1DmToLSuJsrcZeZC1HUdkuaelqPa2Zf2sgbzPYawK/Lh6D87p7RLTdgpIfarwFsGNZTL0Nx5pceTYa7ZF24aKp3eKVzJWV3jQLanbvhYHgtQI/2IkXMji0zyEc5tZ005zHKjwK2lPD276UdcVpuDmWLEhV7W/IfeIk+VPgch/P/XwTS3IHTS3JB7gHDv3mFvbsZ2FIxA2+H5hfcsXvyWcKmrEJ8BDL16rNHinUROq8XC7gnsUwesKQWj/eS7et9Ji/R+MoB20fOFI1WXWBXzI4b10HB3gN3wNa4DzKticJceTxi7ARL+1HDyD/ckYTPKMtLgf2KyZ/eJjp5SbkMXUvG2bTm8JrMhN527t25+2SGzHtzSJxtIyKRZiM02GusnI+S/Ilm9pO7aS5O/FXtjMmNy+9pWw30+mhErzK1jrEsJPGmIQ2inRVmXFAU2Dd7xIc0T5tILXWcWjccv8cyV8fW5vZtVgTPTA6gA1eSF1/lM0u+Rn0hO1N1kmc=",
            "torrent": {
                "url": "https://images.bluehorizon.network/319c8f683bbeb29aef988d02f73dd4fd31c79111.torrent",
                "images": [
                    {
                        "file": "33d56f3c3bbf2d746da5e07589645e5fa28ccc3a.tar.gz",
                        "signature": "gGvzi55sOv8st2wNJqcMdm7cjW7/CYbcyruAkTO8aU8IdWctA9ydgLeFnwkFs9uRvW/SEuNboY2PgwJn6Fkr4KAgQWMGjWMkwnmvw0sk4nhtD5o4nrNgWfBXppaDEQ5at9KoDv2QnQmO43DOGrxoktHtIdF5ip7bqni7hgW1V44lKIOfuFAwAvMCbPuwVf4OQT787d7pFXIVgmti0PHEanYS5vrrT5A4wFoYhFJanCjz0pe6qVsXCyOG6LXYpYYXm1eUA4lNN4kCYFUSUmczdEpw/megeBmtwb/DHl2LbHrAsvF6TEwcXxNVe800GLrd2dI5UTpOQ9mvLFWreLXRbTDJU1GkAilzBq7RmyqB30URtA36A+gUuAPVnYhMg4AEVZxWmbXpvZNl+YxqMZ9b2S7ta8Y4CvnzedDg6YEzPSBgMimjMJJEoQHgmVm9RnLj54dcOM0/5I+dqMc9/l8KtoBIFPnir8i1Sjrh5rJLAvy54uhhTkpOfRLbmm/NQK4Iy8A9/V1i6MZwdyQ6Jmgi6YZRR4ulEfWggC6Q8vQ/ei467NhDi7XgLtZlIze0Kst/sMb0ElJGCtKqKKXmvO6eXcNlmZa3ItTNC5vjsckY8C//xqzT5mw3GlsssDcPyu4hjSDW2FbGXMPWA8mopXRDwnGVYVh/2R6dM2GsQZtauWo="
                    },
                    {
                        "file": "7b0e75dec828496a439ea270d5a975dd443754f4.tar.gz",
                        "signature": "nz5Ef9OwgeMAWoWSLB7QG7U8kOcXTvYo99v9ohbaDU9B81W+RTTa8Wj6OYU0lTnb44W5LFyVNcZxY/CHhmrDogL96C/XCCZJJQ9iobgitGN5ItG7Qw9PKhzQTVyTRAN2JYHcGnsxvjsFHvWN5i52rP6jixd/6Gr5guMGAAWHqUORLHXZdrczwjzzzfay1qfbYBMVj89lZn2qMDo2G5LcyZkf2hYn7IhUQ0kPDo3D35lHJr02kudIZnWd5SFOilJCmjBNHujuskW5RKrd472uBuVTOd3UCETvL7d09TPyFJoyMT+U/fcacSZF9s+NWF5iH5+MzEonHhVBeIkmcrDmnUm7pklKEv0fCnT8UW1rsXT6SNgOa4WT2MMRxFLgJZRDH+CbkwuuQPinwPU3+iituaxqjevw7n53AWMNYJ86P2vkGTkp+ImsFfqBM0tSfJf8zO/u0FZHGJO7PLQjN0KvGPFfAyUvuJxIio0qxAxnpPwZ82V7vWr6x3NSFyKOcpDwixJJalvcIz+BkNgh8lTJa1kdPuYDaWDMIA86BHcjyKKstcUtS366cAWhHHMacvNe/IpLc0aO3z/2hFR2HzeEyax3dNsLEi3IDE2b9A0rQMTwj2CsD4JIqI8YkzggJgCqZVxzXI5Iw+71VSj7UamMJS0DB+ZPL3PAIJx4wTSvE/A="
                    },
                    {
                        "file": "d8406bb962b7bb838e48589a814dd8d2427f11af.tar.gz",
                        "signature": "R0KaIskt9zQZzB0Fr97Otmsngdnnh8zGrqP06Tp+Kzon0d1Oi24CIgSH0T/BBKvMkKOvWWcsq/oNw2CwNnfuyJ6EyDnTf6M0mAy7ZyOcIhB5tVU+S/VibH4CzzfhHM16waSsXDuKTVP0UtrRwvUF+rkuMg861LLRwu39bK7bDOIZaNtCMuQZawc+9snD4cjdAxpD9wdHmMrbuqz4Ol6E7Hp2i1+7qCsZqF2MINtnaw33BuwI9Z+kOnSt4Ni5qJIE1/kYcjB3QjdvoayE98i41kMfi6QuLOKqKP74iJlN8/JCESIqZd2cWZoa6oxFZgsF3nHZqknyjj5N9N9cA63+OHCTft9iiFf0Bf+7SqHJtDaVXDZS0AfEnuZz/XBAD8gy314isutR6mfCgU6DJJXwyIxYWwH9qOJ1wlICJ+rOjBbKSsopPx1wlkklyQpZ6+0diRZcE6RTTsNfCw9jOp2IWGpRoPeb87NuTM3qtRx/9lYxY9W+Bvesb8Pf/hOVC4xCoiC5zUfSCeICLdafxl1KVKQrk0NnfwvkXchN6EW1sb5COzdEZTeyMOOVcN3rNmDvn6iQNo8IcqGkRrpl2CeM2c472S+v5Dnc+hvD7sCtR+RvRCDNIY9APfXrH4HDZADArQh62Gl2aiTGVuGSNXpjulnV3t/Ec2+3bPX38W1WVsk="
                    }
                ]
            }
        }
    ]
  stg: |-
    [
        {
            "matchgroups": [
                [
                    {
                        "attr": "is_bandwidth_test_enabled",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "arm"
                    }
                ]
            ],
            "deployment": "{\"netspeed5\":{\"image\":\"summit.hovitos.engineering/armhf/netspeed5:v1.7\",\"links\":[\"pitcherd\"],\"environment\":[\"DEPL_ENV=staging\"],\"restart\":\"always\"},\"pitcherd\":{\"image\":\"summit.hovitos.engineering/armhf/pitcherd:volcano\",\"environment\":[\"MTN_CATCHERS=https://catcher.staging.bluehorizon.hovitos.engineering:443\",\"MTN_PITCHERD_PORT=8081\"],\"restart\":\"always\"}}",
            "deployment_signature": "llaNF0ReWiIGZQTaLza2sU0a1LjraLwU50dc0AvtWoXGDidDSEEwPBBxjqaSH6YJRUlmuTj6Act5iHiY1ZnQ1lldfYcNv5pYv7wuIT0S70GV8/RhLl8qE+EPnT6ZLUR8oK+Efozk92XrEsuzE17H4lig/xtrrcBElA27gx2n00APjUWPvsnCO9Zkvhn4IftGeWW3Oa3MQTVbaURI6AFJaD7NWFq8rkZ5GMgIwV+WXUyqXS79EePgrCVZBWfpzKyBsBfEngpm5Jxh6qm8v+msO4wsjW0nDIhDaOijXeXVWQsEjGOoLCF9Bxh45JOo+4P+Ng7raNw0fOFhyDYVpFBVrxYBlwieOtD8ji69+0MjWIngsOfmjW5GFFfTcBkRuwlLhyY4nHYbfJTQc9rOUutgMvVJXxZHpFGdVlTCAXWnmVdeYj9ZuFMhPaIArSMepOlglP8sy/0TnDJMXZeKv6WlXpMh/nRx/qszroQej+OUUeIrdw9o4pD8QnvQHK0iJ3C1tKcABuIb3/Kbim3Nc7B7LOTVFK/iap3liPqOEBAl9TofQqyTSJbpP5tbPN8ztFc92Ptvezd4UQuk0X2Mlh+g4aR8HJ1VaXgovoepbh0tfYcDyaN1mMo6XInakxuEGwDc2zGRknUCY86sXLKoLWbRRqhP4w6mFXRF9DimnaCHF3I=",
            "torrent": {
                "url": "https://images.bluehorizon.network/fa6ac28804107342a6514fc48001b40ddc6accc1.torrent",
                "images": [
                    {
                        "file": "c20c0035364fc26f61d7246dcfdadfdda559d672.tar.gz",
                        "signature": "enWOZSWbbu7OTmhACw4BKpCWE1ZiqkvtPf22TIr3CdhWYVmVu0RcsGVma51iWOvhOqMVB1mkfJjJ9hR+4iyIczYqoYSVZ6Vm+Md+ejO/vPXs74aDe7ViNI8OpbiFhiqknH5VqrAHj+/+Ywg6TWDqq2MayMLLnb1hmtWuRm2mUis2TldNCwwcDC10tJhUy7oBoKHVhaT/pE8aHmlvpQ3piz5zmq3QVtnqsapl4Wuqwrxkp/pCEZQ8mLDxqKursvDYFSKOreVlZgT+oKeTl6pzyMw3XfV6Z4Y6OGPfV5vzsZ6TwWdgUuoFVbq8CZmRbd4rPd3JCBkujZJMT+qTwmb4RfQo4EXOU5Kdl52+LtBcmH24Bk5nUyB/Vp8p3Guvc681PhqSy+ruRxedlIYYKFqB8EUuSqj7kgChs8fErVlnDEjBAjxRVRL6F1b1pUg17oTVQktOmZeGYHrnarCUPGwspzD89XHHNkrDh713p+yDHH67E1XOSe5afann58z9SKEEZIgXVMZwmWIxAnGSKKwnG7xsm3I7x4xN9Q77q9dXb3ldPvFXesLaVH+JbRIDBRrZrQBo0c0MiluN0nv+4fJDQpLrkoBFZI5ewbRR8b+Dj8iN2/T48PQbWKkJDdirC9g/giihQS8jIqDGG4OIUoMMWroWMcRzx7TdOKQse95EzpM="
                    },
                    {
                        "file": "e9d7d36b0f6689882c5d5e54e99c6f9836d897e3.tar.gz",
                        "signature": "Mv0zkJ9F4BdWKizPQoob6TruH7kn5nUAG2ew3yquAvjAj06wN6Kqa8b6EoCjH6V855xijULNZ5MsDhMyEUY4V/ZoLGVH1xc8KjUmMbHaA4dCcoLwjrbTdWkuwY0GOQwYiVKUKSQHF3HnlhcDn4uE3Vdj5K3jgC4bFhq0ilx8v2w71sim40Ap9DyX7RZCacX2lw0QV2uiZj3a/l9fC9/QaX/1kqyjBeUFUyHHN1kA7Ak7xvwUbAAnl0th3O211aF9vmq+VwiqNtnunJyzxP/f7z4olI4cpkdp0w9SUGjUyE0fSxrZ4DbJCPj4mEf1jKI6/5PpFntaHiWxc+t+lSb4NZ5TmgZIzt6mkxhwYs8+2i+hhw6qdQ3bVZFupKwyp2i+JUY3q5syU6+unJ5DC6xT2k3h9SmXFPFs/+YuWB/b7bRxZPQlj+DEKpTb/dEzwywcrY+p+gtOInPxJI3J6G8kuV5tKvZ8Xbw5kMP+V5etOBHQqJ7AOlS0/6vrxYkSpFIJ1SZ5NkEA9bRGnrY92oioWeGzxmt+kCnAJ7FXkTa7Zsb2MerTFRhlVeon83bVO4KIGHqgoOqw/GhkPIru3oXsnuY021yYkMSEmD1KzHirnFx39SdIEcWZmQh1nqAkbGuEiBJp2FpXBGNFLE2p0AanSfbWg3lBH0+1czUo2TF+NAw="
                    }
                ]
            }
        },
        {
            "matchgroups": [
                [
                    {
                        "attr": "is_bandwidth_test_enabled",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "amd64"
                    }
                ]
            ],
            "deployment": "{\"netspeed5\":{\"image\":\"summit.hovitos.engineering/x86/netspeed5:volcanostaging\",\"links\":[\"pitcherd\"],\"restart\":\"always\"},\"pitcherd\":{\"image\":\"summit.hovitos.engineering/x86/pitcherd:volcano\",\"environment\":[\"MTN_CATCHERS=https://catcher.staging.bluehorizon.hovitos.engineering:443\",\"MTN_PITCHERD_PORT=8081\"],\"restart\":\"always\"}}",
            "deployment_signature": "P/7SNYtdirIBgXzQHd9Jbm7bz/rA1/RDPBg461Yfxli0SzdfGFakvzpYdQrq/9Vvj2W26dFwcltUgeeQ+R0SU83eCbbmjRAxJuZfDW1Od0lXPDZ5gSRKY8k4yBjrTFi65acpHqtHT9phAUR3W9f3YwFh68dJ1hVfCWEV9ss3Dp0QALSEtXkAIMG1iPgbMAn2DPRx9U1C1RsYkE1wPGFDdyrvPQj82ChD6lTrGmMrWdLwGEm9aAW6nfMgKZ4anXm14X4e4oyX9rQ/h6iE8z0PUJWsuJRHNGuS1W7PRGnOg3cCk4zUi5gyc5qjq8EGawdzSpLcUOEqZ9HSz23cpiqtBSKbjX3F59NRd/6tRlJV+He0LyUnGNtnkHpgBFzZJIXFxMBHrSX+Xn7h6rAxBxh7ShVeNhRK1rHkLjvazpEz33DJp38VnALetZkluzj85gg50fLLFq+K6/CYrVUWHrpRqV0y6lgSUDYHXzuXP+ocAutTqV3qaYUAnE7cCYybXJRRqzkAMf0tMlUp4mkPEYbNARJiYz8J736hnOGy9+PwkyFS/nBJlFlRKxU4IcyfitWTFTvGgeLMw26rumMXEnHELZSu2SfV5bxXHbqLCeMLnOks8yQZLbrhubnFDdnXIWxHpgiDx3smuvXYpOZlaKxa5DSz5iWI84fH2JKfC4SZZok=",
            "torrent": {
                "url": "https://images.bluehorizon.network/f3a39c78edf78d415ff7e634e396d8e34b1656a3.torrent",
                "images": [
                    {
                        "file": "6fcc9d89326a42d48fa596e9f61dba5730b7a20e.tar.gz",
                        "signature": "vB4gfRur+epAbUgDwXoJMuSUMSKoFylNPWk/lQXMv42fYUDjo+he4Ch/P8sxCPVSivAg390yY6TlycBQzh0cNU0MKC6+mpos+tcnUPesjEqVQ7umtNm7Pc559brAQD7VNpL7ge3yTfMBLDozyyYptRXI9bsbzOfI5d61YjOOkCwFxOhvEnvyhuadEBISDkBL4h8yWqFNgEl4R3VuWHnR9vKVeAPdpVbKYKwSbT2Wah91k240P0PTZ3ewxG9vBVshFVL+HJ/3Nr3kZt0YTefhekAor4xT5EVhPj+ZZCqrIcU0unDzvo3Fm7KYctNEOLYl2gwf3GPC9oxRfLMD/5b4BELc/8iS9xpT8711HoykQwOWbh4p3QO1Z9RJQbQ5i5wwxm88O+vebYhjI1Hf4EqvhNEhwQqGbpoEGxP4ickJU/e41aHa5TzLW639hpOsf9EehVVQ8z1ii/lLQKVHo2kU3Ii2XUea6Dj6VlXQw2hjlYgWJBKKiGMoKjL+Ls67lFfIb/GOJ5xYguIZOUI4E6JyCxtR0Fnd/xbyjAiSS3gUw3BzCfXdgyM8aQ1RZkrVSInsT/hr9JG9btuq30e/f1rJgNiGMGgo9i3oWMuF//jwxohyeJZktFUoGU2CFxCNLx84U0hkrs/ERAibCBqdEVHPmxMOBGghXwBm+wzoNpU133M="
                    },
                    {
                        "file": "aef3b0fe6536092014cb29e9723ccc661d321d35.tar.gz",
                        "signature": "EZt30z7roB8nofT1sO7yDpMl5CPWkKRj6e4ZmAF2qPv1JYg/XZA97Z7yvEYH6e5cJHiU7zRaw0Xiw8uZHaBA9hwkN7PHBSYcgqCe5RbXvofylmjBt9lR9xccEFszEl0hk0eMss2YC7UC/5jqkA7oyZc+vu1F1J95r7o0ZACLQ28CeRqwPexNvJbqH3IWIXSwKeaFRrVkzycM9lbp5Z+aRdfCgiCHWz2F/zqs4Bsln7mR2ICF8cwD+VdOaJaCv3qfq3Ec0ze5I7zSkO6rpv/lhxaX/ICdaic5Imzbn86eLxPDkv8qRKqS31AXsERLP7JFrMrjqpej8ZcjA9m6SqF3eTDTlQGiOofrBxYPB4AM6ICQuVZdymjvVkdE4RcxCddKrO0N7P1ICrsnSMqnf9/qhcDZJgy819bQOOJMqOTG/y97pAi0g2cRQO4yWkQWSJihBhz6lkarg/UR/226HNh2IE3CmUVJ14NTBx0woXDIGHQ6cGDMZlHYdvWF7tYXtiRw2PwCnN+QGQCLl7T6k8PhaujGJmByUduV00w6z13ZV87DntOrvHAefE2uLFKeZ1mKgRIoRYtQcxDkIeoCiA+re7oOTu+ZRpOQLu+RVSisE0W8nYhxivTP0byPkHeXi9TAEfZzAlWs5TFbX+VKAn1sHNWAf+0IbK6tBdeiPQXf+Ck="
                    }
                ]
            }
        },
        {
            "matchgroups": [
                [
                    {
                        "attr": "pws",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "arm"
                    }
                ]
            ],
            "deployment": "{\"culex\":{\"image\":\"summit.hovitos.engineering/armhf/culex:volcano\",\"environment\":[\"MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod\"],\"restart\":\"always\"},\"fwweather\":{\"image\":\"summit.hovitos.engineering/armhf/fwweather:volcano\",\"privileged\":true,\"environment\":[\"MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod\"],\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"links\":[\"culex\"],\"restart\":\"always\"},\"eaweather\":{\"image\":\"summit.hovitos.engineering/armhf/eaweather:v1.7\",\"environment\":[\"MTN_MQTT_TOKEN=ZZbrT4ON5rYzoBi7H1VK3Ak9n0Fwjcod\",\"DEPL_ENV=staging\"],\"links\":[\"culex\"],\"restart\":\"always\"}}",
            "deployment_signature": "bUfQLOtVdAtiJVkp1DTR3A/q3eQT8d8qdIYqcAjEK4PUC8baOCLW1iheWRkfq6sHWmyfMiY1cp0sysRq0HZoBmynlA8Tw+0y7gS3Fz2Xn1DKw/xq31ubVoswfVv/Zocg1QI98u+RHAlhj/m3Jyp9pAYAa8Xq0Te14CZ5QDrOUXbddlUu+AuWbnEXVHQKxjFA75zuT7nq/SCRnShKhnBHgwcpxzIy4nA4kQeOdvWd685t8hn6j73tSwi715modCc4SdfjAp5OINfmA1VPycELtarU0BmjFPu51x59vfCHpRLmNBevL1bfhT3gK8f2/0qf9vd5IAiw+j+ompaseTUersmzSkyCXIb/f8ejxyBHaJqsx/uE8sk7fOz1QcSBjbU57XrNwW/B4QKoowwa4V9IDbZqFCjYABxrQhDbt0QKIDI5Qle9BFkpqZCFDMLM5l+q+S3emD1qu3h/pGgrTf6+RDIZgQqQaAQG6uMcREjVh7puvexu/GC7byJCNSSlsG/0bLwsusXYO8AWajwGRIpzJ/5OixlkuIOMHjyIy4qbzOF4oz3jTxKP0x5dqNveICGZVX8z8P679wOEYhc3qPHEzCMOWEfs7n6fIYrKL4FMJZTyEhGU7QzaJ9eOAuIwXA3NdGc96llg94SwEL6VZ32qkLFbWJjPb5G05YsakbdJ4pw=",
            "torrent": {
                "url": "https://images.bluehorizon.network/9f9479d7af3bad137b1dc27e36cd0649e061997c.torrent",
                "images": [
                    {
                        "file": "5048dfbabd3d9c0d2258ca8973f60107cc752f0a.tar.gz",
                        "signature": "AMWA9QKVIzBZpS0XiwuWfU4xKqzrUN4L8lDDPwzYeEP9SgCHJ3awL5pHhZmEYjmwxQ2+zfC5XQDjUFwJi5MSTEkiif/LIKbT7MltSSX9JnanqjilJCkrLynByZB0rsKDuKllLdLOE/6L98XguCkw8pNzT3Uu+aDvbVM5a3Cnvm2mBP9MhdKQRp22KPEswN46MDvGg5V5XtP16CmQt4BXZHxwBbE7IDY/E57Lbp4lvg7xT8KLd0jx4IFlZgW+DMTX0AZuooFsLgTauJ/FuZEKySn1rGGhdjch7sfW/jAbkrEx3BIJ14KOIdkvxn3OnY4RdDkmh+EQFv8NP55Qwi2iLMgEFfF/UwV4ikMTIp21h3zVDSv5MyOnMKMKQNrV6yOmuhXeUZ7uF+dTTGXO5700iRpVxyfOWBizu1geTPo/Pdt0OJqvYvpD5t7uoADkNnodS3DuRP/p004nkRARtRDBzXe91MsxoGqODSsp18/Br72dfGNkSJT9n4fwfXWfU+Fumds37nhuafNLjzLixXRFzW77ypYKwzbiF/+vhTQdYVnkK8FMlhb90Vu1U6FeVhjulp3Nnlg7rzJsRrIN4KSU9QHRiXE6h4j4C/dGWIt5KzH/ohJrkA/KggV2b1N/xzt0fg958vZzuyF23zSqPxPJEjBNqq8LiUFVNdJuobGHgfc="
                    },
                    {
                        "file": "b5804cdedba31c9f37b982d068d53805af80111f.tar.gz",
                        "signature": "fixoIZJ9pUqjiu0t18IcfEMyxUVrt9BzS5t5xX8yibDrNTDBP9uWDXoc2DUWVQX8DyP7/rtlQWD/Uxp1Arm189KfHCTJaRFTNwtGf1uJXBWjlBrpJKo3oLIDoKyr88CA7enloyAlmuBJUpfpgoHaMdqzvoR+NwyRxrRDYATz4HtR8SQcL5OQKRVk2wiy8Yo+qYIrK/4XoYHJCyZUPHSAig5cUCmJeNsgM01OcYtt6Uqu9gL04gJTlAZ7uwsWvfTPwvwB014d3pcUuRLNh9/RACSvc0gwsuZhLEwF1TeH7AJDsccrpgXXWMDbfc4BhWDonetHaPuHAn0WPgBNly+EMv3V7S9KTUBn9KV4+qXzauW5e+AypD9kp70BrgFi6NkRHagpKNAbhqKdEf/j/zlWX23n9GMVjCbeeXnP/7mYsnh7At36GQLJiQVJ0j2bVaTF1gVUfdTpm5LPKF1xNS+XzCSbqkgYhCBXxko6UcfGfwEHhapOLohFsunCRdjPkQkedVDWwKMovM/1CIDT3iY3AAURC5+iEoC8rocSrgc3sBt9NXSk2NM0OMhqwtHkirb/3uEjkcerjSFoQVsSYLNCMxvL0Y5ysksRE64Yxc51wnY58v4wj32Xhto1hFrV6ACSuhTQ6F/BSrCxcKoLnYMpHgHJ1WnkmH01IXbXunUEN+I="
                    },
                    {
                        "file": "ed2db46031edf41f13d74578cf30009bbb218761.tar.gz",
                        "signature": "H/nYpRMHkB14lwXJJFOGyx9GO6wEy0xaV1xS8CCzVaEGTSHp9qmxNY9JJArblORoz5Aev3m3yFRf5NZgK2mUj48LHAxfqJYTl8MfpegyXUORYdSWL8y3dgPaYrXBTMiOYr7+pk+BsUyh9w+EHj2OnWUPZe0y/VNpqi5CsTZqBrgEOERHjC49GTQ5L11Z1KXDpT3HN0Z070UVUyjAcXrPkQdJvRz7g6ZBDOFdqvuUDskfCP+A7Ie5Tjkw9E7KxpUK3z04EJB4Qf5WfOrOEW6UZZtZDCIK6dKBCXq19tmzqawa/9fapWGBG+ptKprMHzI8XTcccHlnRW65pUZi7FM6wIea4t9an73lUxMDVSqcc/enTDsDpMTj3FOXHqppavCK91ZncWTn9UzFjFLQ3GCoVEPe67jUldVlZirev0fy8EdC7XTLnd5fUnY66/ebP5rDruMwjrxsxpf7utVp3NRAFqF/teVcZ6pcqseHpdZejRM3sDrXzFsn8AkN/k90reDH9ym5blYNL9J/PFXXYGvVv3empDVgGEqSn4RC7DaUjAccDKNIuYGT9j2OPj1Piv+iitVsvF02Enlmu04QRrkWPDXPwPXf16jUikdkAkWY/8GQ+JYLCDy4kH+fOVGzgENKqyVz8+4h3cfrX8JzGS3Y1jIViZeWrG1m9Jixsr9jGso="
                    }
                ]
            }
        },
        {
            "matchgroups": [
                [
                    {
                        "attr": "sdr",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "arm"
                    }
                ]
            ],
            "deployment": "{\"rtlsdr\":{\"image\":\"summit.hovitos.engineering/armhf/rtlsdr:volcano\",\"privileged\":true,\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"restart\":\"always\"},\"apollo\":{\"image\":\"summit.hovitos.engineering/armhf/apollo:v1.7\",\"links\":[\"rtlsdr\"],\"environment\":[\"DUMP1090_HOST=staging.bluehorizon.hovitos.engineering\",\"DEPL_ENV=staging\"],\"restart\":\"always\"}}",
            "deployment_signature": "u3WGdXmss96vJu7kjPsQjQfD8eKBXqq8S9jQXR/jntPVFT5NO5jSBB6QSFhzgAj2gpEkZlJLLel0EjT9MmL2Z/zAr3Pfimw46H1mWj+p4ETU4e95lkudJdpzY+1GRkPJqZ9cFT15nvhN7xvo9GdawFPTGQlWWC9wJNZDJ34ropW2lmWqNUTX1AUKfS24TZynL34FFfSwBjs1UwAcHS2xDic50VEPm8Hyk2m4a0KrtCb2NEM0RSe2V5S8N4jVsZZlYDNf1iUj+USpK53jKGJwoAWKchvLN+aI7UAYvMyvjzXcCcnrZRxQnPJmU9gNx0NrdyhGSoWUPsa2y0xKgvZoZpJ5g43gIrVSTbyHpow5XeVnEbz64CMHIk29ZKzgxUkW1UZpZhIICJflmE/F9YqxCeyRBk+TNGhLkt1ckSyYstQNTNoa4Ngl0rVlVzZZOeOlCDpXf/9YAZsvSzam6xbauaORxu/PCjJXUoS2g62by/b4xPK3Rq5DvtmOsN1WHblZ6IvrJved2h7iWrLyqHHSFTTQZ36oP0KAovWwCONDyjTwiNRmwr7waaXXGCsGI7fKul55Lblw008nDu0Fpshf3ANlwemSvpTRYNEblPQCXSL2n2e0ivNnyiixJpPGfGDFURk9p3EpVMx3qI4qDVhfqNba/wNpZxv069TAKtux/+g=",
            "torrent": {
                "url": "https://images.bluehorizon.network/da20b92d15c69649feb9d7cf8ea56eb2a3f219af.torrent",
                "images": [
                    {
                        "file": "6141732c10f50b768f2101d7bdc13d70a2a9b35c.tar.gz",
                        "signature": "g/G7Z4cA73BqMBmhBmSt9bbUDYIBeVHeF3DXCycwGNqTbCoodCTtE/WIQDTdxsElGto4r61ZZjwcBQ9UXltBTU/PEdsbSXF3tmZ/lDSBzMraNTP58EesiXdyb5yq0vUUnRtpMp23Qdd+JitSzLwpyxWCWVZ5oObFWYjezgYh6NCdTe9SK/lumvt7nlrAWmgdoQH9NXRv47hzezrnqwuOwsSQAh/THL0idBmGKQ1JBmva8Ysp8sSVSZ4zeKD56cx7h72oTGs0+j7pMlpzT1R4dzb+wxZUhF3stqKenkmRaZfju4LhVqJ4KYdZqs6sJHzT+ENOWwK15pYKfj8Cdq5/rApd0T4qPImLofipvJ9SkPvmaNXYWVwBdzvE+A/7FAcePlPQ49aGqn9WE/+ksRsdu+98m0HQne0oUpHr+NdJZSlegQmbR/Q6Pd/JV+Pgk6bJQxCQfR3mEjAhxU29ppP4Csne/Y2Qub4kAQSSZoSSW0PUq+yDUghd3kRdDHWsQY0JXJ2SkTgogzzjuF801xfjm8F5LJd0g29i++ifzR9PeP1yob6mzNZKKCjsTU0jH5+fdLTT5+yZKC7Wn518qYE5e6TG613J0fGhd74aBcmtESA1oKiZMhWbOSWYrHgHXJRFFU9lM38c/tXKVcukFRpngUhnrTs0srXZotW70CuQ+qM="
                    },
                    {
                        "file": "8e14433f1bbe65e490b101abd9bfc74164069d55.tar.gz",
                        "signature": "h8h+lnPv0vAnSiSWuzaPixMOTvngMxecVjZpQbI688n6qqzR2hkSa3ibdtQ29p1NFVD53wJJv7Boks8Eq5W/r+7h7lM35pFcie6XxkqaLLshO4OklV3NqvBWUbKyGFgmJkTuUvXC+chjuUxZWFsrKp6VHE/MEggjuIQsVP2erBsKRANF3rWp3nfhITeV1s02fLiZO+e77fqOUDwo6cCab6+uqo0D44HsvZOgg9y3yF9zJdR5FGa/9WSJyfstC/tCi4eFr/vBw4tQBT9CpAa2M/Pn6s8eHVKg7SG24jZQKchrsZ/vE225dJQLvfpvS+REFk3gASIsG7MaC1yvSmer7D7H58dF396LcDmfAZnlOFsBBXUKBKAgDoiko8w7fXf/3m3HoplgW67uGIGOn2fL+6dCAKp41u4YgruxJVQZC6PxTJV+2xMuhF8TK2P6DTQaz0fDWvzU31DpjcpOSYkHoqnNot7K8O6emTG/U617RjcnitWJ7hecodwlBDlwK/hYoLsuEBWXwx0sTTlGpRyHebpMZkoYxb5dfsGwW34KEIneutI3T1GnOc1HPi3KmjcYcX+vncJCQVaZShIvJxqTUwoCg8HOe5Xrq0nGHNwPXlbC/lDhLkMLjCCuQcARYv9QllDYV3VKlQIs25CJ6K6Ht+UE4lRAHrQLKBYZppeKXvg="
                    }
                ]
            }
        },
        {
            "matchgroups": [
                [
                    {
                        "attr": "dyn_pvp",
                        "value": "true"
                    }
                ],
                [
                    {
                        "attr": "arch",
                        "value": "arm"
                    }
                ]
            ],
            "deployment": "{\"pvp\":{\"image\":\"summit.hovitos.engineering/armhf/pvp:volcano\",\"cap_add\":[\"NET_RAW\"],\"restart\":\"always\"}}",
            "deployment_signature": "RAWs8T4e0slizJr0YiQ5uB0dI9tWaaUdADqWevT2TpBKAvCo8UOB6oNMnF8CISh/ScBP+3LOugxsjLoJSRHWSHoS1EkoddA7ypudChVYy0MljS+HzUWuP/aehaYKlRQ3a/CQp3UvWNaacN6MCu4hQXQNJbPsW7HhcPTHAOeZaQpIXCAH3fq7L+2DFTvvIHu9KOx8683jYbRboSChS9l4g/csMFUiA/8r1rPI/pNu8rHj8UXRzBzs+8qijrUSn9Xprq3wH2PAKObjoaGzuRQhE3mbqExzsBo3+LtJ+ayS5g93r1gqgvFKVw9xNnBggyTuqMlN5ZAAU5DQUcHOa+3ZUFs5JwkEI47mlcxCDU1X98abuTWX6AQTRr5fZ1ycJSPu7VnOGr5i7qQThMZqRyUN7mnWui+HOKnshddc8wVOH+mMF/jAwJT21A+47ZN6nLO5anxgp3moZDH5si2Bm17NnVWthFzOnYoEj+kfoe9Qz9gerErc1cUMi/OlZZx4+iIjn8pSPuug2W+Ju5irZTPmhtlOqyHwvdNqTX98HJXMpwsNdGA6xNRC6qRjkArufVEMeLhvQfjW3LkPTx9kkuyxZYhejpucY8kS0wCDsXpZLieF13K4DBR0QKBLXJeql16IhPJNyrqKfBZUeu8T4WqP4DRbS87CmUurb77VXEQ3YUw=",
            "torrent": {
                "url": "https://images.bluehorizon.network/71006aadd76974b9d8f0d6005ae57a8eed12c006.torrent",
                "images": [
                    {
                        "file": "71006aadd76974b9d8f0d6005ae57a8eed12c006.tar.gz",
                        "signature": "NdeljkWyN7AjPYI/3X4h15Xi06UnyhTVaZJwhoHHU1E1wMCwRGL0Oc651N4nOlcsCSnbQM0j/bRqu3Jb0vO1Zg2+qjEvnt6KAbk2Ger65uSnwL9vmPjGEpb5QdnJRsFl/g8K0+J4C1VDufxJ0OPflD1PPloopfN3CNXtIZbZhtII76F7kDunAEupaeGkBoQbYE9qSXSZMqFoArejKmUOQlR2BbnNtLWwXiWfwnNBg7M3U9F9ERu9i/jjME7WF8u+ZV5YD7tJSXT+5tfbzQbzlBpWA6x5ODvG4NHFitLZrtLKAZphsMp3h959RE8Kny5B5S54ZF03BzkneoRxf9kuH3If4uOy0+fEPMQ9UR520K4tsr/iW+XlVviuDeU9D3IDASIDBlu1xSZTMIoP03lmOk1hNnX5gyKrW9uPuJZDTbyB1JQ/uZvjsiFaUB5sRgcilQ4t+jcK7oafiOKVt6QXJ9YYQuBqx5eb2YsDEa3iEEnVrvDFcG5E2elYcx/GeQnM8N5HrcEhtdf7xyIPwzjZiqguaN2nOnRUHHJy4LWo+b6ciKeR1XQjKUflP73AfHniPL/uon6eF1CSZA3TuLDuRxkmJnTTCPiCRcCnlNtNx7y+CiFPeYTTCK/X4xISg0+kDgrGFGwkC0U/8TiXGBH8lfN+bcs/b2I4pic8YxVcG/Y="
                    }
                ]
            }
        }
    ]

# vim: set ts=4 sw=4 expandtab:
