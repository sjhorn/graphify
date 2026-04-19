module DataProcessor where

import Data.List (sort, group)
import qualified Data.Map as Map

-- | Calculate statistics from a list of numbers
calculateStats :: [Int] -> (Int, Int, Int)
calculateStats xs = (minimum xs, maximum xs, sum xs)

-- | Process data with configuration
processData :: Config -> [Int] -> [Int]
processData config xs =
    let filtered = filter (> 0) xs
    in sort filtered

-- | Main data processor function
runProcessor :: String -> IO ()
runProcessor inputFile = do
    contents <- readFile inputFile
    let stats = calculateStats (map read (lines contents))
    print stats

-- | Data configuration type
data Config = Config {
    verbose :: Bool,
    threshold :: Int
} deriving (Show, Eq)

-- | Create a new config
createConfig :: Bool -> Int -> Config
createConfig v t = Config v t
