// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.17;

contract Storage{
    event StorageUpdate(uint256 previousValue, uint256 newValue);

    uint256 number;

    function store(uint256 num) public{
        emit StorageUpdate(number, num);
        number = num;
    }

    function retrieve() public view returns (uint256){
        return number;
    }
}

