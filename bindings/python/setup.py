from setuptools import setup, find_packages

setup(
    name="topospatia",
    version="1.0.0",
    description="Python bindings for TopoSpatiaDB: Multidimensional Sheaf Holographic DBMS",
    author="Tao Science Foundation",
    license="BSD-3-Clause",
    packages=find_packages(),
    package_data={"topospatia": ["*.so"]},
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: BSD License",
        "Operating System :: POSIX :: Linux",
        "Topic :: Database :: Database Engines/Servers",
    ],
    python_requires=">=3.8",
)
